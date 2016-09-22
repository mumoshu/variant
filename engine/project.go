package engine

import (
	"fmt"
	"os"
	"path"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/juju/errors"
	bunyan "github.com/mumoshu/logrus-bunyan-formatter"
	"github.com/spf13/viper"

	// TODO Eliminate the tight-coupling with cobra
	"github.com/spf13/cobra"
)

type Project struct {
	Name                string
	CommandRelativePath string
	FlowDefs            map[string]*FlowDef
	CachedFlowOutputs   map[string]interface{}
	Verbose             bool
	Output              string
	Env                 string
}

func (p Project) Reconfigure() {
	if p.Verbose {
		log.SetLevel(log.DebugLevel)
	}

	commandName := path.Base(os.Args[0])
	if p.Output == "bunyan" {
		log.SetFormatter(&bunyan.Formatter{Name: commandName})
	} else if p.Output == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	} else if p.Output == "text" {
		log.SetFormatter(&log.TextFormatter{})
	} else if p.Output == "message" {
		log.SetFormatter(&MessageOnlyFormatter{})
	} else {
		log.Fatalf("Unexpected output format specified: %s", p.Output)
	}

}

func (p *Project) AllVariables(flowDef *FlowDef) []*Variable {
	return p.CollectVariablesRecursively(flowDef.Key, "")
}

func (p *Project) CollectVariablesRecursively(currentFlowKey FlowKey, path string) []*Variable {
	result := []*Variable{}

	ctx := log.WithFields(log.Fields{"prefix": fmt.Sprintf("%s", currentFlowKey.String())})

	currentFlowDef, err := p.FindFlowDef(currentFlowKey)

	if err != nil {
		allFlowDefs := []string{}
		for _, t := range p.FlowDefs {
			allFlowDefs = append(allFlowDefs, t.Key.String())
		}
		ctx.Debugf("is not a FlowDef in: %v", allFlowDefs)
		return []*Variable{}
	}

	for _, input := range currentFlowDef.Inputs {
		childKey := p.CreateFlowKeyFromInput(input)

		ctx.Debugf("depends on %s", childKey.String())

		vars := p.CollectVariablesRecursively(childKey, fmt.Sprintf("%s.", currentFlowKey.String()))

		for _, v := range vars {
			result = append(result, v)
		}

		variable := &Variable{
			FlowKey:     currentFlowKey,
			FullName:    fmt.Sprintf("%s.%s", currentFlowKey.String(), input.Name),
			Name:        input.Name,
			Parameters:  input.Parameters,
			Description: input.Description,
			Candidates:  input.Candidates,
			Complete:    input.Complete,
		}

		ctx.WithFields(log.Fields{"full": variable.FullName, "flow": variable.FlowKey.String()}).Debugf("has var %s. short=%s", variable.Name, variable.ShortName())

		result = append(result, variable)
	}

	return result
}

func (p Project) CreateFlowKey(flowKeyStr string) FlowKey {
	c := strings.Split(flowKeyStr, ".")
	return FlowKey{Components: c}
}

func (p Project) CreateFlowKeyFromVariable(variable *Variable) FlowKey {
	return p.CreateFlowKeyFromInputName(variable.Name)
}

func (p Project) CreateFlowKeyFromInput(input *Input) FlowKey {
	return p.CreateFlowKeyFromInputName(input.Name)
}

func (p Project) CreateFlowKeyFromInputName(inputName string) FlowKey {
	c := strings.Split(p.Name+"."+inputName, ".")
	return FlowKey{Components: c}
}

func (p Project) RunFlowForKeyString(keyStr string, args []string, caller ...FlowDef) (string, error) {
	flowKey := p.CreateFlowKey(fmt.Sprintf("%s.%s", p.Name, keyStr))
	return p.RunFlowForKey(flowKey, args, caller...)
}

func (p Project) RunFlowForKey(flowKey FlowKey, args []string, caller ...FlowDef) (string, error) {
	provided := p.GetValueForConfigKey(flowKey.ShortString())

	if provided != "" {
		log.Debugf("Output for flow %s is already provided in configuration: %s", flowKey.ShortString(), provided)
		log.Info(provided)
		return provided, nil
	}

	flowDef, err := p.FindFlowDef(flowKey)

	if err != nil {
		return "", errors.Annotate(err, "RunFlowError")
	}

	vars := map[string](interface{}){}
	vars["args"] = args
	vars["env"] = p.Env
	vars["cmd"] = p.CommandRelativePath

	inputs, err := p.AggregateVariablesOfFlowForKey(flowKey, args, caller...)

	if err != nil {
		return "", errors.Annotatef(err, "Flow `%s` failed", flowKey.String())
	}

	for k, v := range inputs {
		vars[k] = v
	}

	flow := &Flow{
		Key:         flowKey,
		ProjectName: flowDef.ProjectName,
		Steps:       flowDef.Steps,
		Vars:        vars,
		Autoenv:     flowDef.Autoenv,
		Autodir:     flowDef.Autodir,
		Interactive: flowDef.Interactive,
		FlowDef:     flowDef,
	}

	log.Debugf("Flow: %v", flow)

	output, error := flow.Run(&p, caller...)

	log.Debugf("Output: %s", output)

	if error != nil {
		error = errors.Annotatef(error, "Flow `%s` failed", flowKey.String())
	}

	return output, error
}

func (p Project) AggregateVariablesOfFlowForKey(flowKey FlowKey, args []string, caller ...FlowDef) (map[string]interface{}, error) {
	aggregated := map[string]interface{}{}
	if err := p.CollectVariablesOfFlowForKey(flowKey, aggregated, args, caller...); err != nil {
		return nil, errors.Annotatef(err, "One or more inputs for flow %s failed", flowKey.String())
	}
	if err := p.CollectVariablesOfParent(flowKey, aggregated); err != nil {
		return nil, errors.Annotatef(err, "One or more inputs for flow %s failed", flowKey.String())
	}
	return aggregated, nil
}

type AnyMap map[string]interface{}

func (p Project) CollectVariablesOfParent(flowKey FlowKey, aggregated AnyMap) error {
	parentKey, err := flowKey.Parent()
	if err != nil {
		log.Debug("%v", err)
	} else {
		if err := p.CollectVariablesOfFlowForKey(*parentKey, aggregated, []string{}); err != nil {
			return errors.Annotatef(err, "AggregateInputsForParent(%s) failed", flowKey.String())
		}
		if err := p.CollectVariablesOfParent(*parentKey, aggregated); err != nil {
			return errors.Annotatef(err, "AggregateInputsForParent(%s) failed", flowKey.String())
		}
	}
	return nil
}

func (p Project) GetValueForConfigKey(k string) string {
	ctx := log.WithFields(log.Fields{"prefix": k})

	lastIndex := strings.LastIndex(k, ".")

	provided := ""

	if lastIndex != -1 {
		a := []rune(k)
		k1 := string(a[:lastIndex])
		k2 := string(a[lastIndex+1:])

		values := viper.GetStringMapString(k1)

		ctx.Debugf("viper.GetStringMap(k1=%s)=%v, k2=%s", k1, values, k2)

		if values != nil && values[k2] != "" {
			provided = values[k2]
			return provided
		}
	}

	provided = viper.GetString(k)
	ctx.Debugf("viper.GetString(\"%s\") #=> \"%s\"", k, provided)

	return provided
}

func (p Project) CollectVariablesOfFlowForKey(flowKey FlowKey, variables AnyMap, args []string, caller ...FlowDef) error {
	var initialFlowKey string
	if len(caller) > 0 {
		initialFlowKey = caller[0].Key.ShortString()
	} else {
		initialFlowKey = ""
	}

	if initialFlowKey != "" {
		log.Debugf("Collecting inputs for the flow `%v` via the flow `%s`", flowKey.ShortString(), initialFlowKey)
	} else {
		log.Debugf("Collecting inputs for the flow `%v`", flowKey.ShortString())
	}

	flowDef, err := p.FindFlowDef(flowKey)
	if err != nil {
		return errors.Trace(err)
	}
	for i, input := range flowDef.Variables {
		log.Debugf("Flow `%v` depends on the input `%s`", flowKey.ShortString(), input.ShortName())
		ctx := log.WithFields(log.Fields{"prefix": input.Name})

		var arg *string
		if len(args) >= i+1 {
			ctx.Debugf("positional argument provided: %s", args[i])
			arg = &args[i]
		}

		var provided string

		if initialFlowKey != "" {
			provided = p.GetValueForConfigKey(fmt.Sprintf("%s.%s", initialFlowKey, input.ShortName()))
		}

		if provided == "" && strings.LastIndex(input.ShortName(), flowKey.ShortString()) == -1 {
			provided = p.GetValueForConfigKey(fmt.Sprintf("%s.%s", flowKey.ShortString(), input.ShortName()))
		}

		if provided == "" {
			provided = p.GetValueForConfigKey(input.ShortName())
		}

		pathComponents := strings.Split(input.Name, ".")

		if arg != nil {
			SetValueAtPath(variables, pathComponents, *arg)
		} else if provided == "" {
			var output interface{}
			var err error
			if output, err = FetchCache(p.CachedFlowOutputs, pathComponents); output == nil {
				output, err = p.RunFlowForKey(p.CreateFlowKeyFromVariable(input), []string{}, *flowDef)
				if err != nil {
					return errors.Annotatef(err, "Missing value for input `%s`. Please provide a command line option or a positional argument or a flow for it`", input.ShortName())
				}
				SetValueAtPath(p.CachedFlowOutputs, pathComponents, output)
			}
			if err != nil {
				return errors.Trace(err)
			}
			SetValueAtPath(variables, pathComponents, output)
		} else {
			SetValueAtPath(variables, pathComponents, provided)
		}

	}
	return nil
}

func (p *Project) FindFlowDef(flowKey FlowKey) (*FlowDef, error) {
	t := p.FlowDefs[flowKey.String()]

	if t == nil {
		return nil, errors.Errorf("No FlowDef exists for the flow key `%s`", flowKey.String())
	}

	return t, nil
}

func (p *Project) RegisterFlowDef(flowKey FlowKey, flowDef *FlowDef) {
	p.FlowDefs[flowKey.String()] = flowDef
}

func (p *Project) GenerateCommand(flowConfig *FlowConfig, rootCommand *cobra.Command, parentFlowKey []string) (*cobra.Command, error) {
	positionalArgs := ""
	for i, input := range flowConfig.Inputs {
		if i != len(flowConfig.Inputs)-1 {
			positionalArgs += fmt.Sprintf("[%s ", input.Name)
		} else {
			positionalArgs += fmt.Sprintf("[%s", input.Name)
		}
	}
	for i := 0; i < len(flowConfig.Inputs); i++ {
		positionalArgs += "]"
	}

	var cmd = &cobra.Command{

		Use: fmt.Sprintf("%s %s", flowConfig.Name, positionalArgs),
	}
	if flowConfig.Description != "" {
		cmd.Short = flowConfig.Description
		cmd.Long = flowConfig.Description
	}

	flowKeyStr := strings.Join(append(parentFlowKey, flowConfig.Name), ".")
	flowKey := p.CreateFlowKey(flowKeyStr)
	flowDef := &FlowDef{
		Key:         flowKey,
		Inputs:      flowConfig.Inputs,
		ProjectName: p.Name,
		Steps:       flowConfig.Steps,
		Autoenv:     flowConfig.Autoenv,
		Autodir:     flowConfig.Autodir,
		Interactive: flowConfig.Interactive,
		FlowConfig:  flowConfig,
		Command:     cmd,
	}
	p.RegisterFlowDef(flowKey, flowDef)

	if len(flowConfig.Steps) > 0 {
		cmd.Run = func(cmd *cobra.Command, args []string) {
			p.Reconfigure()

			log.Debugf("Number of inputs: %v", len(flowConfig.Inputs))

			if _, err := p.RunFlowForKey(flowKey, args); err != nil {
				c := strings.Join(strings.Split(flowKey.String(), "."), " ")
				stack := strings.Split(errors.ErrorStack(err), "\n")
				for i := len(stack)/2 - 1; i >= 0; i-- {
					opp := len(stack) - 1 - i
					stack[i], stack[opp] = stack[opp], stack[i]
				}
				log.Errorf("Command `%s` failed\n\nCaused by:\n%s", c, strings.Join(stack, "\n"))
				log.Debugf("Stack:\n%v", errors.ErrorStack(errors.Trace(err)))
				os.Exit(1)
			}
		}
	}

	if rootCommand != nil {
		rootCommand.AddCommand(cmd)
	}

	log.WithFields(log.Fields{"prefix": flowKey.String()}).Debug("is a flow")

	p.GenerateCommands(flowConfig.FlowConfigs, cmd, append(parentFlowKey, flowConfig.Name))

	return cmd, nil
}

func (p *Project) GenerateCommands(flowConfigs []*FlowConfig, rootCommand *cobra.Command, parentFlowKey []string) (*cobra.Command, error) {
	for _, c := range flowConfigs {
		p.GenerateCommand(c, rootCommand, parentFlowKey)
	}

	return rootCommand, nil
}

func (p *Project) GenerateAllFlags() {
	for _, flowDef := range p.FlowDefs {
		flowDef.Variables = p.AllVariables(flowDef)
		for _, input := range flowDef.Variables {
			log.Debugf("Configuring flag and config key for flow %s's input: %s", flowDef.Key.String(), input.Name)

			flowConfig := flowDef.FlowConfig
			cmd := flowDef.Command
			var description string
			if input.Description != "" {
				description = input.Description
			} else {
				description = input.Name
			}

			var name string
			if input.FlowKey.String() == flowDef.Key.String() {
				name = input.Name
			} else {
				name = input.ShortName()
			}

			log.Debugf("short=%s, full=%s, name=%s, selected=%s", input.ShortName(), input.FullName, input.Name, name)

			flagName := strings.Replace(name, ".", "-", -1)

			var longerName string
			if input.FlowKey.ShortString() == flowDef.Key.ShortString() {
				longerName = input.ShortName()
			} else {
				longerName = fmt.Sprintf("%s.%s", flowDef.Key.ShortString(), input.ShortName())
			}

			if len(flowConfig.FlowConfigs) == 0 {
				cmd.Flags().StringP(flagName, "" /*string(input.Name[0])*/, "", description)
				//log.Debugf("Binding flag --%s to the config key %s", flagName, name)
				//viper.BindPFlag(name, cmd.Flags().Lookup(flagName))
				log.Debugf("Binding flag --%s of the command %s to the input %s with the config key %s", flagName, flowDef.Key.ShortString(), input.Name, longerName)
				viper.BindPFlag(longerName, cmd.Flags().Lookup(flagName))
			} else {
				cmd.PersistentFlags().StringP(flagName, "" /*string(input.Name[0])*/, "" /*default*/, description)
				//log.Debugf("Binding persistent flag --%s to the config key %s", flagName, name)
				//viper.BindPFlag(name, cmd.PersistentFlags().Lookup(flagName))
				log.Debugf("Binding persistent flag --%s to the config key %s", flagName, longerName)
				viper.BindPFlag(longerName, cmd.PersistentFlags().Lookup(flagName))
			}
		}
	}
}
