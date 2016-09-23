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

type Application struct {
	Name                string
	CommandRelativePath string
	Flows               map[string]*Flow
	CachedFlowOutputs   map[string]interface{}
	Verbose             bool
	Output              string
	Env                 string
}

func (p Application) UpdateLoggingConfiguration() {
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

func (p *Application) ResolveInputsForFlow(flowDef *Flow) []*ResolvedInput {
	return p.ResolveInputsForFlowKey(flowDef.Key, "")
}

func (p *Application) ResolveInputsForFlowKey(currentFlowKey FlowKey, path string) []*ResolvedInput {
	result := []*ResolvedInput{}

	ctx := log.WithFields(log.Fields{"prefix": fmt.Sprintf("%s", currentFlowKey.String())})

	currentFlow, err := p.FindFlow(currentFlowKey)

	if err != nil {
		allFlows := []string{}
		for _, t := range p.Flows {
			allFlows = append(allFlows, t.Key.String())
		}
		ctx.Debugf("is not a Flow in: %v", allFlows)
		return []*ResolvedInput{}
	}

	for _, input := range currentFlow.Inputs {
		childKey := p.CreateFlowKeyFromInput(input)

		ctx.Debugf("depends on %s", childKey.String())

		vars := p.ResolveInputsForFlowKey(childKey, fmt.Sprintf("%s.", currentFlowKey.String()))

		for _, v := range vars {
			result = append(result, v)
		}

		variable := &ResolvedInput{
			FlowKey:  currentFlowKey,
			FullName: fmt.Sprintf("%s.%s", currentFlowKey.String(), input.Name),
			Input:    *input,
		}

		ctx.WithFields(log.Fields{"full": variable.FullName, "flow": variable.FlowKey.String()}).Debugf("has var %s. short=%s", variable.Name, variable.ShortName())

		result = append(result, variable)
	}

	return result
}

func (p Application) CreateFlowKey(flowKeyStr string) FlowKey {
	c := strings.Split(flowKeyStr, ".")
	return FlowKey{Components: c}
}

func (p Application) CreateFlowKeyFromResolvedInput(variable *ResolvedInput) FlowKey {
	return p.CreateFlowKeyFromInputName(variable.Name)
}

func (p Application) CreateFlowKeyFromInput(input *Input) FlowKey {
	return p.CreateFlowKeyFromInputName(input.Name)
}

func (p Application) CreateFlowKeyFromInputName(inputName string) FlowKey {
	c := strings.Split(p.Name+"."+inputName, ".")
	return FlowKey{Components: c}
}

func (p Application) RunFlowForKeyString(keyStr string, args []string, caller ...Flow) (string, error) {
	flowKey := p.CreateFlowKey(fmt.Sprintf("%s.%s", p.Name, keyStr))
	return p.RunFlowForKey(flowKey, args, caller...)
}

func (p Application) RunFlowForKey(flowKey FlowKey, args []string, caller ...Flow) (string, error) {
	provided := p.GetValueForConfigKey(flowKey.ShortString())

	if provided != "" {
		log.Debugf("Output for flow %s is already provided in configuration: %s", flowKey.ShortString(), provided)
		log.Info(provided)
		return provided, nil
	}

	flowDef, err := p.FindFlow(flowKey)

	if err != nil {
		return "", errors.Annotate(err, "RunFlowError")
	}

	vars := map[string](interface{}){}
	vars["args"] = args
	vars["env"] = p.Env
	vars["cmd"] = p.CommandRelativePath

	inputs, err := p.InheritedInputValuesForFlowKey(flowKey, args, caller...)

	if err != nil {
		return "", errors.Annotatef(err, "Flow `%s` failed", flowKey.String())
	}

	for k, v := range inputs {
		vars[k] = v
	}

	flow := &BoundFlow{
		Vars: vars,
		Flow: *flowDef,
	}

	log.Debugf("Flow: %v", flow)

	output, error := flow.Run(&p, caller...)

	log.Debugf("Output: %s", output)

	if error != nil {
		error = errors.Annotatef(error, "Flow `%s` failed", flowKey.String())
	}

	return output, error
}

func (p Application) InheritedInputValuesForFlowKey(flowKey FlowKey, args []string, caller ...Flow) (map[string]interface{}, error) {
	result := map[string]interface{}{}

	// TODO make this parents-first instead of children-first?
	direct, err := p.DirectInputValuesForFlowKey(flowKey, args, caller...)

	if err != nil {
		return nil, errors.Annotatef(err, "One or more inputs for flow %s failed", flowKey.String())
	}

	for k, v := range direct {
		result[k] = v
	}

	parentKey, err := flowKey.Parent()

	if err == nil {
		inherited, err := p.InheritedInputValuesForFlowKey(*parentKey, []string{}, caller...)

		if err != nil {
			return nil, errors.Annotatef(err, "AggregateInputsForParent(%s) failed", flowKey.String())
		}

		for k, v := range inherited {
			result[k] = v
		}
	}

	return result, nil
}

type AnyMap map[string]interface{}

func (p Application) GetValueForConfigKey(k string) string {
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

func (p Application) DirectInputValuesForFlowKey(flowKey FlowKey, args []string, caller ...Flow) (map[string]interface{}, error) {
	values := map[string]interface{}{}

	var baseFlowKey string
	if len(caller) > 0 {
		baseFlowKey = caller[0].Key.ShortString()
	} else {
		baseFlowKey = ""
	}

	if baseFlowKey != "" {
		log.Debugf("Collecting inputs for the flow `%v` via the flow `%s`", flowKey.ShortString(), baseFlowKey)
	} else {
		log.Debugf("Collecting inputs for the flow `%v`", flowKey.ShortString())
	}

	flowDef, err := p.FindFlow(flowKey)
	if err != nil {
		return nil, errors.Trace(err)
	}
	for i, input := range flowDef.ResolvedInputs {
		log.Debugf("Flow `%v` depends on the input `%s`", flowKey.ShortString(), input.ShortName())
		ctx := log.WithFields(log.Fields{"prefix": input.Name})

		var arg *string
		if len(args) >= i+1 {
			ctx.Debugf("positional argument provided: %s", args[i])
			arg = &args[i]
		}

		var provided string

		if baseFlowKey != "" {
			provided = p.GetValueForConfigKey(fmt.Sprintf("%s.%s", baseFlowKey, input.ShortName()))
		}

		if provided == "" && strings.LastIndex(input.ShortName(), flowKey.ShortString()) == -1 {
			provided = p.GetValueForConfigKey(fmt.Sprintf("%s.%s", flowKey.ShortString(), input.ShortName()))
		}

		if provided == "" {
			provided = p.GetValueForConfigKey(input.ShortName())
		}

		pathComponents := strings.Split(input.Name, ".")

		if arg != nil {
			SetValueAtPath(values, pathComponents, *arg)
		} else if provided == "" {
			var output interface{}
			var err error
			if output, err = FetchCache(p.CachedFlowOutputs, pathComponents); output == nil {
				output, err = p.RunFlowForKey(p.CreateFlowKeyFromResolvedInput(input), []string{}, *flowDef)
				if err != nil {
					return nil, errors.Annotatef(err, "Missing value for input `%s`. Please provide a command line option or a positional argument or a flow for it`", input.ShortName())
				}
				SetValueAtPath(p.CachedFlowOutputs, pathComponents, output)
			}
			if err != nil {
				return nil, errors.Trace(err)
			}
			SetValueAtPath(values, pathComponents, output)
		} else {
			SetValueAtPath(values, pathComponents, provided)
		}

	}
	return values, nil
}

func (p *Application) FindFlow(flowKey FlowKey) (*Flow, error) {
	t := p.Flows[flowKey.String()]

	if t == nil {
		return nil, errors.Errorf("No Flow exists for the flow key `%s`", flowKey.String())
	}

	return t, nil
}

func (p *Application) RegisterFlow(flowKey FlowKey, flowDef *Flow) {
	p.Flows[flowKey.String()] = flowDef
}

func (p *Application) GenerateFlow(flowConfig *FlowConfig, parentFlowKey []string) (*Flow, error) {
	flowKeyComponents := append(parentFlowKey, flowConfig.Name)
	flowKeyStr := strings.Join(flowKeyComponents, ".")
	flowKey := p.CreateFlowKey(flowKeyStr)
	flow := &Flow{
		Key:         flowKey,
		ProjectName: p.Name,
		//Command:     cmd,
		FlowConfig: *flowConfig,
	}
	p.RegisterFlow(flowKey, flow)

	flows := []*Flow{}

	for _, c := range flow.FlowConfigs {
		f, err := p.GenerateFlow(c, flowKeyComponents)

		if err != nil {
			return nil, errors.Trace(err)
		}

		flows = append(flows, f)
	}

	flow.Flows = flows

	return flow, nil
}

func (p *Application) GenerateCommand(flow *Flow, rootCommand *cobra.Command) (*cobra.Command, error) {
	positionalArgs := ""
	for i, input := range flow.Inputs {
		if i != len(flow.Inputs)-1 {
			positionalArgs += fmt.Sprintf("[%s ", input.Name)
		} else {
			positionalArgs += fmt.Sprintf("[%s", input.Name)
		}
	}
	for i := 0; i < len(flow.Inputs); i++ {
		positionalArgs += "]"
	}

	var cmd = &cobra.Command{
		Use: fmt.Sprintf("%s %s", flow.Name, positionalArgs),
	}
	if flow.Description != "" {
		cmd.Short = flow.Description
		cmd.Long = flow.Description
	}

	flowKey := flow.Key

	if len(flow.Steps) > 0 {
		cmd.Run = func(cmd *cobra.Command, args []string) {
			p.UpdateLoggingConfiguration()

			log.Debugf("Number of inputs: %v", len(flow.Inputs))

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

	for _, f := range flow.Flows {
		p.GenerateCommand(f, cmd)
	}

	flow.Command = cmd

	return cmd, nil
}

func (p *Application) ResolveInputs() {
	for _, flow := range p.Flows {
		flow.ResolvedInputs = p.ResolveInputsForFlow(flow)
	}
}

func (p *Application) GenerateAllFlags() {
	for _, flow := range p.Flows {
		for _, input := range flow.ResolvedInputs {
			log.Debugf("Configuring flag and config key for flow %s's input: %s", flow.Key.String(), input.Name)

			flowConfig := flow.FlowConfig
			cmd := flow.Command
			var description string
			if input.Description != "" {
				description = input.Description
			} else {
				description = input.Name
			}

			var name string
			if input.FlowKey.String() == flow.Key.String() {
				name = input.Name
			} else {
				name = input.ShortName()
			}

			log.Debugf("short=%s, full=%s, name=%s, selected=%s", input.ShortName(), input.FullName, input.Name, name)

			flagName := strings.Replace(name, ".", "-", -1)

			var longerName string
			if input.FlowKey.ShortString() == flow.Key.ShortString() {
				longerName = input.ShortName()
			} else {
				longerName = fmt.Sprintf("%s.%s", flow.Key.ShortString(), input.ShortName())
			}

			if len(flowConfig.FlowConfigs) == 0 {
				cmd.Flags().StringP(flagName, "" /*string(input.Name[0])*/, "", description)
				//log.Debugf("Binding flag --%s to the config key %s", flagName, name)
				//viper.BindPFlag(name, cmd.Flags().Lookup(flagName))
				log.Debugf("Binding flag --%s of the command %s to the input %s with the config key %s", flagName, flow.Key.ShortString(), input.Name, longerName)
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
