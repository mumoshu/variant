package run

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/juju/errors"
	"github.com/spf13/viper"

	"github.com/mumoshu/variant/cmd"
	"github.com/mumoshu/variant/pkg"
	"github.com/mumoshu/variant/pkg/cli/env"
	"github.com/mumoshu/variant/pkg/util/envutil"
	"github.com/mumoshu/variant/pkg/util/fileutil"
	"github.com/spf13/cobra"
	"path"
)

func init() {
	log.SetOutput(os.Stdout)

	verbose := false
	logtostderr := false
	for _, e := range os.Environ() {
		if strings.Contains(e, "VERBOSE=") {
			verbose = true
			break
		}
		if strings.Contains(e, "LOGTOSTDERR=") {
			logtostderr = true
			break
		}
	}

	if verbose {
		log.SetLevel(log.DebugLevel)
	}

	if logtostderr {
		log.SetOutput(os.Stderr)
	}

	variant.Register(variant.NewTaskStepLoader())
	variant.Register(variant.NewScriptStepLoader())
	variant.Register(variant.NewOrStepLoader())
	variant.Register(variant.NewIfStepLoader())
}

func Dev() {
	var taskDef *variant.TaskDef
	var args []string

	var cmdName string
	var cmdPath string
	var varfile string

	if len(os.Args) > 1 && (os.Args[0] != "var" || os.Args[0] != "/usr/bin/env") && fileutil.Exists(os.Args[1]) {
		varfile = os.Args[1]
		args = os.Args[2:]
		cmdPath = varfile
		cmdName = path.Base(cmdPath)
	} else {
		cmdPath = os.Args[0]
		cmdName = path.Base(cmdPath)
		varfile = fmt.Sprintf("%s.definition.yaml", cmdName)
		args = os.Args[1:]
	}

	environ := envutil.ParseEnviron()

	if environ["VARFILE"] != "" {
		varfile = environ["VARFILE"]
	}

	if !fileutil.Exists(varfile) {
		varfile = "Variantfile"
	}

	if fileutil.Exists(varfile) {
		taskConfigFromFile, err := variant.ReadTaskDefFromFile(varfile)

		if err != nil {
			log.Errorf("%+v", err)
			panic(errors.Trace(err))
		}
		taskDef = taskConfigFromFile
	} else {
		taskDef = variant.NewDefaultTaskConfig()
	}

	taskDef.Name = cmdName

	RunTaskDef(cmdPath, taskDef, args)
}

func YAML(yaml string) {
	cmdPath := os.Args[0]
	cmdName := path.Base(cmdPath)

	taskDef, err := variant.ReadTaskDefFromBytes([]byte(yaml))

	if err != nil {
		log.Errorf("%+v", err)
		panic(errors.Trace(err))
	}

	taskDef.Name = cmdName

	RunTaskDef(cmdPath, taskDef, os.Args[1:])
}

func RunTaskDef(commandPath string, rootTaskConfig *variant.TaskDef, args []string) {
	var err error

	var envFromFile string
	commandName := rootTaskConfig.Name
	envFromFile, err = env.New(commandName).GetOrSet("dev")
	if err != nil {
		panic(errors.Trace(err))
	}

	taskNamer := variant.NewTaskNamer(commandName)

	g := variant.NewTaskCreator(taskNamer)

	rootTask, err1 := g.Create(rootTaskConfig, []string{}, commandName)
	if err1 != nil {
		panic(err1)
	}

	taskRegistry := variant.NewTaskRegistry()
	taskRegistry.RegisterTasks(rootTask)

	inputResolver := variant.NewRegistryBasedInputResolver(taskRegistry, taskNamer)
	inputResolver.ResolveInputs()

	p := &variant.Application{
		Name:                commandName,
		CommandRelativePath: commandPath,
		CachedTaskOutputs:   map[string]interface{}{},
		Verbose:             false,
		Output:              "text",
		Env:                 envFromFile,
		TaskNamer:           taskNamer,
		TaskRegistry:        taskRegistry,
		InputResolver:       inputResolver,
		Viper:               viper.GetViper(),
	}

	adapter := variant.NewCobraAdapter(p)

	rootCmd, err := adapter.GenerateCommand(rootTask, nil)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	rootCmd.AddCommand(cmd.EnvCmd)
	rootCmd.AddCommand(cmd.InitCmd)
	rootCmd.AddCommand(cmd.VersionCmd(log.StandardLogger()))

	rootCmd.PersistentPostRunE = func(_ *cobra.Command, _ []string) error {
		return p.UpdateLoggingConfiguration()
	}

	adapter.GenerateAllFlags()

	rootCmd.PersistentFlags().BoolVarP(&(p.Verbose), "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVarP(&(p.Output), "output", "o", "text", "Output format. One of: json|text|bunyan")
	rootCmd.PersistentFlags().BoolVarP(&(p.Colorize), "color", "C", true, "Colorize output")
	rootCmd.PersistentFlags().StringVarP(&(p.ConfigFile), "config-file", "c", "", "Path to config file")
	rootCmd.PersistentFlags().BoolVar(&(p.LogToStderr), "logtostderr", true, "write log messages to stderr")

	// see `func ExecuteC` in https://github.com/spf13/cobra/blob/master/command.go#L671-L677 for usage of ParseFlags()
	rootCmd.ParseFlags(args)

	// Workaround: We want to set log level via command-line option before the rootCmd is run
	err = p.UpdateLoggingConfiguration()
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// Deferred to respect output format specified via the --output flag
	//if !varfileExists {
	//	log.Infof("%s does not exist", varfile)
	//}

	if p.ConfigFile != "" {
		viper.SetConfigFile(p.ConfigFile)

		if err := viper.MergeInConfig(); err != nil {
			log.Errorf("%v", err)
			panic(err)
		}
	} else {
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")

		// See "How to merge two config files" https://github.com/spf13/viper/issues/181
		viper.SetConfigName(commandName)
		commonConfigFile := fmt.Sprintf("%s.yaml", commandName)
		commonConfigMsg := fmt.Sprintf("loading config file %s...", commonConfigFile)
		if fileutil.Exists(commonConfigFile) {
			if err := viper.MergeInConfig(); err != nil {
				log.Errorf("%serror", commonConfigMsg)
				panic(err)
			}
			log.Debugf("%sdone", commonConfigMsg)
		} else {
			log.Debugf("%smissing", commonConfigMsg)
		}
	}

	env.SetAppName(commandName)
	envMsg := fmt.Sprintf("loading env file %s...", env.GetPath())
	envName, err := env.Get()
	if err != nil {
		log.Debugf("%smissing", envMsg)
	} else {
		log.Debugf("%sdone", envMsg)

		envConfigName := fmt.Sprintf("config/environments/%s", envName)
		envConfigFile := fmt.Sprintf("%s.yaml", envConfigName)
		envConfigMsg := fmt.Sprintf("loading config file %s...", envConfigFile)
		viper.SetConfigName(envConfigName)
		if fileutil.Exists(envConfigFile) {
			if err := viper.MergeInConfig(); err != nil {
				log.Errorf("%serror", envConfigMsg)
				panic(err)
			}
			log.Debugf("%sdone", envConfigMsg)
		} else {
			log.Debugf("%smissing", envConfigMsg)
		}
	}

	//Set the environment prefix as app name
	viper.SetEnvPrefix(strings.ToUpper(commandName))
	viper.AutomaticEnv()

	//Substitute the . and - to _,
	replacer := strings.NewReplacer(".", "_", "-", "_")
	viper.SetEnvKeyReplacer(replacer)

	//	var rootCmd = &cobra.Command{Use: c.Name}

	cobraApp := &CobraApp{
		viperCfg: viper.GetViper(),
		cobraCmd: rootCmd,
	}

	if err := cobraApp.Run(args); err != nil {
		switch cmdErr := err.(type) {
		case variant.CommandError:
			c := strings.Join(strings.Split(cmdErr.TaskName.String(), "."), " ")
			if log.GetLevel() == log.DebugLevel {
				log.Errorf("Stack trace: %+v", err)
			}
			errs := strings.Split(err.Error(), ": ")
			msg := strings.Join(errs, "\n")
			log.Errorf("Error: `%s` failed: %s", c, msg)
			if strings.Trim(cmdErr.Cause, " \n\t") != "" {
				log.Errorf("Caused by: %s", cmdErr.Cause)
			}
		default:
			log.Errorf("Unexpected type of error %T: %s", err, err)
		}
		os.Exit(1)
	}
}
