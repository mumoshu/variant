package variant

import (
	"fmt"
	"github.com/juju/errors"
	"github.com/mumoshu/variant/pkg/cli/env"
	"github.com/mumoshu/variant/pkg/util/fileutil"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"strings"
)

type CobraApp struct {
	viperCfg *viper.Viper
	cobraCmd *cobra.Command
}

func (a *CobraApp) Run(args []string) error {
	a.cobraCmd.SetArgs(args)
	return a.cobraCmd.Execute()
}

type Opts struct {
	CommandPath string
	Args        []string
	Log         *logrus.Logger

	ExtraCmds []*cobra.Command
}

func Init(rootTaskConfig *TaskDef, opts ...Opts) (*CobraApp, error) {
	var o Opts
	if len(opts) == 0 {
		o = Opts{Args: []string{}}
	} else if len(opts) == 1 {
		o = opts[0]
	} else {
		return nil, fmt.Errorf("unexpected number of opts: %d", len(opts))
	}
	log := o.Log
	if log == nil {
		log = logrus.StandardLogger()
	}

	var err error

	var envFromFile string
	commandName := rootTaskConfig.Name
	envFromFile, err = env.New(commandName).GetOrDefault("dev")
	if err != nil {
		return nil, errors.Trace(err)
	}

	taskNamer := NewTaskNamer(commandName)

	g := NewTaskCreator(taskNamer)

	rootTask, err1 := g.Create(rootTaskConfig, []string{}, commandName)
	if err1 != nil {
		return nil, err1
	}

	taskRegistry := NewTaskRegistry()
	taskRegistry.RegisterTasks(rootTask)

	inputResolver := NewRegistryBasedInputResolver(taskRegistry, taskNamer)
	inputResolver.ResolveInputs()

	v := viper.GetViper()

	p := &Application{
		Name:                commandName,
		CommandRelativePath: o.CommandPath,
		CachedTaskOutputs:   map[string]interface{}{},
		Verbose:             false,
		Output:              "text",
		Env:                 envFromFile,
		TaskNamer:           taskNamer,
		TaskRegistry:        taskRegistry,
		InputResolver:       inputResolver,
		Viper:               v,
		Log:                 log,
	}

	adapter := NewCobraAdapter(p)

	rootCmd, err := adapter.GenerateCommand(rootTask, nil)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	if len(o.ExtraCmds) > 0 {
		rootCmd.AddCommand(o.ExtraCmds...)
	}

	rootCmd.PersistentPostRunE = func(_ *cobra.Command, _ []string) error {
		return p.UpdateLoggingConfiguration()
	}

	adapter.GenerateAllFlags()

	rootCmd.PersistentFlags().BoolVarP(&(p.Verbose), "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVarP(&(p.Output), "output", "o", "text", "Output format. One of: json|text|bunyan")
	rootCmd.PersistentFlags().BoolVarP(&(p.Colorize), "color", "C", true, "Colorize output")
	rootCmd.PersistentFlags().StringVarP(&(p.ConfigFile), "config-file", "c", "", "Path to config file")
	rootCmd.PersistentFlags().BoolVar(&(p.LogToStderr), "logtostderr", true, "write log messages to stderr")

	// Set default log level.
	v.SetDefault("log_level", "info")

	// Set default colors for the logs.
	v.SetDefault("log_color_panic", "red")
	v.SetDefault("log_color_fatal", "red")
	v.SetDefault("log_color_error", "red")
	v.SetDefault("log_color_warn", "red")
	v.SetDefault("log_color_info", "cyan")
	v.SetDefault("log_color_debug", "dark_gray")
	v.SetDefault("log_color_trace", "dark_gray")

	// see `func ExecuteC` in https://github.com/spf13/cobra/blob/master/command.go#L671-L677 for usage of ParseFlags()
	rootCmd.ParseFlags(o.Args)

	// Deferred to respect output format specified via the --output flag
	//if !varfileExists {
	//	log.Infof("%s does not exist", varfile)
	//}

	if p.ConfigFile != "" {
		v.SetConfigFile(p.ConfigFile)

		if err := v.MergeInConfig(); err != nil {
			return nil, err
		}
	} else {
		v.SetConfigType("yaml")
		v.AddConfigPath(".")

		// See "How to merge two config files" https://github.com/spf13/viper/issues/181
		v.SetConfigName(commandName)
		commonConfigFile := fmt.Sprintf("%s.yaml", commandName)
		commonConfigMsg := fmt.Sprintf("loading config file %s...", commonConfigFile)
		if fileutil.Exists(commonConfigFile) {
			if err := v.MergeInConfig(); err != nil {
				log.Errorf("%serror", commonConfigMsg)
				return nil, err
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
		v.SetConfigName(envConfigName)
		if fileutil.Exists(envConfigFile) {
			if err := v.MergeInConfig(); err != nil {
				log.Errorf("%serror", envConfigMsg)
				panic(err)
			}
			log.Debugf("%sdone", envConfigMsg)
		} else {
			log.Debugf("%smissing", envConfigMsg)
		}
	}

	//Set the environment prefix as app name
	v.SetEnvPrefix(strings.ToUpper(commandName))
	v.AutomaticEnv()

	//Substitute the . and - to _,
	replacer := strings.NewReplacer(".", "_", "-", "_")
	v.SetEnvKeyReplacer(replacer)

	// Workaround: We want to set log level via command-line option before the rootCmd is run
	err = p.UpdateLoggingConfiguration()
	if err != nil {
		return nil, err
	}

	//	var rootCmd = &cobra.Command{Use: c.Name}

	return &CobraApp{
		viperCfg: v,
		cobraCmd: rootCmd,
	}, nil
}
