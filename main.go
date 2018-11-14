package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	log "github.com/Sirupsen/logrus"

	"github.com/juju/errors"
	"github.com/spf13/viper"

	"github.com/mumoshu/variant/cmd"
	engine "github.com/mumoshu/variant/pkg"
	"github.com/mumoshu/variant/pkg/cli/env"
	"github.com/mumoshu/variant/pkg/steps"
	"github.com/mumoshu/variant/pkg/util/envutil"
	"github.com/mumoshu/variant/pkg/util/fileutil"
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
}

func main() {
	engine.Register(steps.NewTaskStepLoader())
	engine.Register(steps.NewScriptStepLoader())
	engine.Register(steps.NewOrStepLoader())
	engine.Register(steps.NewIfStepLoader())

	var commandName string
	var commandPath string
	var varfile string
	var args []string

	if len(os.Args) > 1 && (os.Args[0] != "var" || os.Args[0] != "/usr/bin/env") && fileutil.Exists(os.Args[1]) {
		varfile = os.Args[1]
		args = os.Args[2:]
		commandName = path.Base(varfile)
		commandPath = varfile
	} else {
		commandName = path.Base(os.Args[0])
		commandPath = os.Args[0]
		varfile = fmt.Sprintf("%s.definition.yaml", commandName)
		args = os.Args[1:]
	}

	environ := envutil.ParseEnviron()

	if environ["VARFILE"] != "" {
		varfile = environ["VARFILE"]
	}

	var rootTaskConfig *engine.TaskDef

	varfileExists := fileutil.Exists(varfile)

	if varfileExists {

		taskConfigFromFile, err := engine.ReadTaskConfigFromFile(varfile)

		if err != nil {
			log.Errorf("%+v", err)
			panic(errors.Trace(err))
		}
		rootTaskConfig = taskConfigFromFile
	} else {
		rootTaskConfig = engine.NewDefaultTaskConfig()
	}

	var err error

	rootTaskConfig.Name = commandName

	var envFromFile string
	envFromFile, err = env.New(rootTaskConfig.Name).GetOrSet("dev")
	if err != nil {
		panic(errors.Trace(err))
	}

	taskNamer := engine.NewTaskNamer(rootTaskConfig.Name)

	g := engine.NewTaskCreator(taskNamer)

	rootTask, err1 := g.Create(rootTaskConfig, []string{}, rootTaskConfig.Name)
	if err1 != nil {
		panic(err1)
	}

	taskRegistry := engine.NewTaskRegistry()
	taskRegistry.RegisterTasks(rootTask)

	inputResolver := engine.NewRegistryBasedInputResolver(taskRegistry, taskNamer)
	inputResolver.ResolveInputs()

	p := &engine.Application{
		Name:                rootTaskConfig.Name,
		CommandRelativePath: commandPath,
		CachedTaskOutputs:   map[string]interface{}{},
		Verbose:             false,
		Output:              "text",
		Env:                 envFromFile,
		TaskNamer:           taskNamer,
		TaskRegistry:        taskRegistry,
		InputResolver:       inputResolver,
	}

	adapter := engine.NewCobraAdapter(p)

	rootCmd, err := adapter.GenerateCommand(rootTask, nil)
	rootCmd.AddCommand(cmd.EnvCmd)
	rootCmd.AddCommand(cmd.VersionCmd(log.StandardLogger()))

	adapter.GenerateAllFlags()

	rootCmd.PersistentFlags().BoolVarP(&(p.Verbose), "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVarP(&(p.Output), "output", "o", "text", "Output format. One of: json|text|bunyan")
	rootCmd.PersistentFlags().StringVarP(&(p.ConfigFile), "config-file", "c", "", "Path to config file")
	rootCmd.PersistentFlags().BoolVar(&(p.LogToStderr), "logtostderr", true, "write log messages to stderr")

	// see `func ExecuteC` in https://github.com/spf13/cobra/blob/master/command.go#L671-L677 for usage of ParseFlags()
	rootCmd.ParseFlags(args)

	// Workaround: We want to set log leve via command-line option before the rootCmd is run
	p.UpdateLoggingConfiguration()

	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// Deferred to respect output format specified via the --output flag
	if !varfileExists {
		log.Infof("%s does not exist", varfile)
	}

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
		viper.SetConfigName(rootTaskConfig.Name)
		commonConfigFile := fmt.Sprintf("%s.yaml", rootTaskConfig.Name)
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

	rootCmd.SetArgs(args)
	rootCmd.Execute()
}
