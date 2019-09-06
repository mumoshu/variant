package variant

import (
	"fmt"
	"github.com/juju/errors"
	"github.com/mumoshu/variant/pkg/cli/env"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"strings"
)

type CobraApp struct {
	VariantApp *Application
	cobraCmd   *cobra.Command
}

func (a *CobraApp) Run(args []string) (map[string]string, error) {
	c := a.cobraCmd

	c.SetArgs(append([]string{}, args...))

	c.SilenceErrors = true
	c.SilenceUsage = true
	cmd, err := a.cobraCmd.ExecuteC()
	if err != nil {
		if cmd != nil {
			c = cmd
		}
		msg := err.Error()
		var usage bool
		if strings.HasPrefix(msg, `unknown command "`) {
			if c.RunE != nil {
				a.cobraCmd.Args = cobra.ArbitraryArgs
				newargs := []string{}
				newargs = append(newargs, args[0], "--")
				newargs = append(newargs, args[1:]...)
				a.cobraCmd.SetArgs(newargs)
				err = a.cobraCmd.Execute()
			} else {
				usage = true
			}
		} else if strings.HasPrefix(msg, `unknown flag: `) ||
			strings.HasPrefix(msg, `unknown shorthand flag: `) ||
			strings.HasPrefix(msg, `bad flag syntax: `) ||
			strings.HasPrefix(msg, `flag needs an argument: `) {

			usage = true
		}

		if usage {
			fmt.Fprintf(os.Stderr, c.UsageString())
			err = InitError{fmt.Errorf("Error: %v\nRun '%v --help' for usage.", err, c.CommandPath())}
		}

		if err != nil {
			return nil, err
		}
	}

	return a.VariantApp.LastOutputs, nil
}

type Opts struct {
	CommandPath string
	Args        []string
	Log         *logrus.Logger

	ExtraCmds []*cobra.Command
}

func Init(commandPath string, rootTaskConfig *TaskDef, opts ...Opts) (*CobraApp, error) {
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

	env.SetAppName(commandName)

	taskNamer := NewTaskNamer(commandName)

	g := NewTaskCreator(taskNamer)

	rootTask, err := g.Create(rootTaskConfig, []string{}, commandName)
	if err != nil {
		return nil, err
	}

	taskRegistry := NewTaskRegistry()
	taskRegistry.RegisterTasks(rootTask)

	inputResolver := NewRegistryBasedInputResolver(taskRegistry, taskNamer)
	inputResolver.ResolveInputs()

	v := viper.GetViper()

	p := &Application{
		Name:                commandName,
		CommandRelativePath: commandPath,
		CachedTaskOutputs:   map[string]interface{}{},
		Env:                 envFromFile,
		TaskNamer:           taskNamer,
		TaskRegistry:        taskRegistry,
		InputResolver:       inputResolver,
		Viper:               v,
		Log:                 log,
		CommandName:         commandName,
	}

	adapter := NewCobraAdapter(p)

	rootCmd, err := adapter.GenerateCommand(rootTask, nil)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	rootCmd.PersistentPostRunE = func(_ *cobra.Command, _ []string) error {
		return p.UpdateLoggingConfiguration()
	}

	adapter.GenerateAllFlags()

	rootCmd.PersistentFlags().BoolVarP(&(p.Verbose), "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVarP(&(p.Output), "output", "o", "text", "Output format. One of: json|text|bunyan")
	rootCmd.PersistentFlags().BoolVarP(&(p.Colorize), "color", "C", true, "Colorize output")
	rootCmd.PersistentFlags().BoolVar(&(p.NoColorize), "no-color", false, "Un-colorize output")
	rootCmd.PersistentFlags().StringVarP(&(p.ConfigFile), "config-file", "c", "", "Path to config file")
	rootCmd.PersistentFlags().BoolVar(&(p.LogToStderr), "logtostderr", true, "write log messages to stderr")
	rootCmd.PersistentFlags().StringArrayVarP(&(p.ConfigContexts), "config-context", "x", []string{}, "Config context")
	rootCmd.PersistentFlags().StringArrayVarP(&(p.ConfigDirs), "config-dir", "d", []string{}, "Config dir")

	rootCmd.PersistentFlags().StringVarP(&(p.LogLevel), "log-level", "", "info", "Log level. One of: panic|fatal|error|warn|info|debug|trace")
	rootCmd.PersistentFlags().StringVarP(&(p.LogColorPanic), "log-color-panic", "", "red", "Log message color: panic")
	rootCmd.PersistentFlags().StringVarP(&(p.LogColorFatal), "log-color-fatal", "", "red", "Log message color: fatal")
	rootCmd.PersistentFlags().StringVarP(&(p.LogColorError), "log-color-error", "", "red", "Log message color: error")
	rootCmd.PersistentFlags().StringVarP(&(p.LogColorWarn), "log-color-warn", "", "red", "Log message color: warn")
	rootCmd.PersistentFlags().StringVarP(&(p.LogColorInfo), "log-color-info", "", "cyan", "Log message color: info")
	rootCmd.PersistentFlags().StringVarP(&(p.LogColorDebug), "log-color-debug", "", "dark_gray", "Log message color: debug")
	rootCmd.PersistentFlags().StringVarP(&(p.LogColorTrace), "log-color-trace", "", "dark_gray", "Log message color: trace")

	// Bind persistent flags to viper
	viper.BindPFlags(rootCmd.PersistentFlags())

	//Substitute the . and - to _,
	replacer := strings.NewReplacer(".", "_", "-", "_")
	v.SetEnvKeyReplacer(replacer)

	// Set the env prefix for global flags.
	v.SetEnvPrefix("VARIANT")
	v.AutomaticEnv()

	// see `func ExecuteC` in https://github.com/spf13/cobra/blob/master/command.go#L671-L677 for usage of ParseFlags()
	rootCmd.ParseFlags(o.Args)

	p.setGlobalParams()

	// Workaround: We want to set log level via command-line option before the rootCmd is run
	err = p.UpdateLoggingConfiguration()
	if err != nil {
		return nil, err
	}

	// Load a config from the provided flag (could be loaded through Viper as well)
	if p.ConfigFile != "" {
		p.loadConfigFile(p.ConfigFile)
	}

	// Load contexts configuration
	p.loadContextConfigs()

	envMsg := fmt.Sprintf("loading env file %s...", env.GetPath())
	envName, err := env.Get()
	if err != nil {
		log.Debugf("%smissing", envMsg)
	} else {
		log.Debugf("%sdone", envMsg)
		envConfigName := fmt.Sprintf("config/environments/%s", envName)
		p.loadConfig(envConfigName)
	}

	// Hide built-in commands in help
	v.SetDefault("hide_extra_cmds", false)
	if len(o.ExtraCmds) > 0 {
		for k, _ := range o.ExtraCmds {
			o.ExtraCmds[k].Hidden = v.GetBool("hide_extra_cmds")
		}
		rootCmd.AddCommand(o.ExtraCmds...)
	}

	//Set the environment prefix as app name
	v.SetEnvPrefix(strings.ToUpper(commandName))

	return &CobraApp{
		VariantApp: p,
		cobraCmd:   rootCmd,
	}, nil
}
