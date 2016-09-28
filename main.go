package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	log "github.com/Sirupsen/logrus"

	"github.com/juju/errors"
	"github.com/spf13/viper"

	"./cli/env"
	"./cmd"
	"./engine"
	"./steps"
	"./util/envutil"
	"./util/fileutil"
)

func init() {
	log.SetOutput(os.Stdout)

	verbose := false
	for _, e := range os.Environ() {
		if strings.Contains(e, "VERBOSE=") {
			verbose = true
			break
		}
	}

	if verbose {
		log.SetLevel(log.DebugLevel)
	}
}

func main() {
	engine.Register(steps.NewFlowStepLoader())
	engine.Register(steps.NewScriptStepLoader())
	engine.Register(steps.NewOrStepLoader())

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

	var rootFlowConfig *engine.FlowConfig

	varfileExists := fileutil.Exists(varfile)

	if varfileExists {

		flowConfigFromFile, err := engine.ReadFlowConfigFromFile(varfile)

		if err != nil {
			log.Errorf(errors.ErrorStack(err))
			panic(errors.Trace(err))
		}
		rootFlowConfig = flowConfigFromFile
	} else {
		rootFlowConfig = engine.NewDefaultFlowConfig()
	}

	var err error

	rootFlowConfig.Name = commandName

	var envFromFile string
	envFromFile, err = env.New(rootFlowConfig.Name).GetOrSet("dev")
	if err != nil {
		panic(errors.Trace(err))
	}

	flowKeyCreator := engine.NewFlowKeyCreator(rootFlowConfig.Name)

	g := engine.NewFlowGenerator(flowKeyCreator)

	rootFlow, err1 := g.GenerateFlow(rootFlowConfig, []string{}, rootFlowConfig.Name)
	if err1 != nil {
		panic(err1)
	}

	flowRegistry := engine.NewFlowRegistry()
	flowRegistry.RegisterFlows(rootFlow)

	inputResolver := engine.NewRegistryBasedInputResolver(flowRegistry, flowKeyCreator)
	inputResolver.ResolveInputs()

	p := &engine.Application{
		Name:                rootFlowConfig.Name,
		CommandRelativePath: commandPath,
		CachedFlowOutputs:   map[string]interface{}{},
		Verbose:             false,
		Output:              "text",
		Env:                 envFromFile,
		FlowKeyCreator:      flowKeyCreator,
		FlowRegistry:        flowRegistry,
		InputResolver:       inputResolver,
	}

	adapter := engine.NewCobraAdapter(p)

	rootCmd, err := adapter.GenerateCommand(rootFlow, nil)
	rootCmd.AddCommand(cmd.EnvCmd)
	rootCmd.AddCommand(cmd.VersionCmd(log.StandardLogger()))

	adapter.GenerateAllFlags()

	rootCmd.PersistentFlags().BoolVarP(&(p.Verbose), "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVarP(&(p.Output), "output", "o", "text", "Output format. One of: json|text|bunyan")

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

	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	// See "How to merge two config files" https://github.com/spf13/viper/issues/181
	viper.SetConfigName(rootFlowConfig.Name)
	commonConfigFile := fmt.Sprintf("%s.yaml", rootFlowConfig.Name)
	commonConfigMsg := fmt.Sprintf("loading config file %s...", commonConfigFile)
	if fileutil.Exists(commonConfigFile) {
		if err := viper.MergeInConfig(); err != nil {
			log.Errorf("%serror", commonConfigMsg)
			panic(err)
		}
		log.Debugf("%sdone", commonConfigMsg)
	} else {
		log.Debugf("%smissing")
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
