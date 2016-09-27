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

	"../util/maputil"
)

type Application struct {
	Name                string
	CommandRelativePath string
	CachedFlowOutputs   map[string]interface{}
	Verbose             bool
	Output              string
	Env                 string
	FlowRegistry        *FlowRegistry
	InputResolver       InputResolver
	FlowKeyCreator      *FlowKeyCreator
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

func (p Application) RunFlowForKeyString(keyStr string, args []string, caller ...Flow) (string, error) {
	flowKey := p.FlowKeyCreator.CreateFlowKey(fmt.Sprintf("%s.%s", p.Name, keyStr))
	return p.RunFlowForKey(flowKey, args, caller...)
}

func (p Application) RunFlowForKey(flowKey FlowKey, args []string, caller ...Flow) (string, error) {
	var ctx *log.Entry

	if len(caller) == 1 {
		ctx = log.WithFields(log.Fields{"caller": caller[0].Key.ShortString()})
	} else {
		ctx = log.WithFields(log.Fields{})
	}

	ctx.Debugf("app started flow %s", flowKey.ShortString())

	provided := p.GetValueForConfigKey(flowKey.ShortString())

	if provided != "" {
		ctx.Debugf("app skipped flow %s via provided value: %s", flowKey.ShortString(), provided)
		ctx.Info(provided)
		return provided, nil
	}

	flowDef, err := p.FlowRegistry.FindFlow(flowKey)

	if err != nil {
		return "", errors.Annotatef(err, "app failed finding flow %s", flowKey.ShortString())
	}

	vars := map[string](interface{}){}
	vars["args"] = args
	vars["env"] = p.Env
	vars["cmd"] = p.CommandRelativePath

	inputs, err := p.InheritedInputValuesForFlowKey(flowKey, args, caller...)

	if err != nil {
		return "", errors.Annotatef(err, "app failed running flow %s", flowKey.ShortString())
	}

	for k, v := range inputs {
		vars[k] = v
	}

	flow := &BoundFlow{
		Vars: vars,
		Flow: *flowDef,
	}

	kv := maputil.FlattenAsString(vars)

	ctx.Debugf("app bound variables for flow %s: %s", flowKey.ShortString(), kv)

	output, error := flow.Run(&p, caller...)

	ctx.Debugf("app received output from flow %s: %s", flowKey.ShortString(), output)

	if error != nil {
		error = errors.Annotatef(error, "app failed running flow %s", flowKey.ShortString())
	}

	ctx.Debugf("app finished running flow %s", flowKey.ShortString())

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
	ctx := log.WithFields(log.Fields{"key": k})

	lastIndex := strings.LastIndex(k, ".")

	provided := ""

	if lastIndex != -1 {
		a := []rune(k)
		k1 := string(a[:lastIndex])
		k2 := string(a[lastIndex+1:])

		values := viper.GetStringMapString(k1)

		ctx.Debugf("app fetched %s: %v", k1, values)

		var provided string

		if values != nil && values[k2] != "" {
			provided = values[k2]
		} else {
			provided = ""
		}

		ctx.Debugf("app fetched %s[%s]: %s", k1, k2, provided)

		if provided != "" {
			return provided
		}
	}

	provided = viper.GetString(k)
	ctx.Debugf("app fetched string %s: %s", k, provided)

	return provided
}

func (p Application) DirectInputValuesForFlowKey(flowKey FlowKey, args []string, caller ...Flow) (map[string]interface{}, error) {
	var ctx *log.Entry

	if len(caller) == 1 {
		ctx = log.WithFields(log.Fields{"caller": caller[0].Key.ShortString(), "flow": flowKey.ShortString()})
	} else {
		ctx = log.WithFields(log.Fields{"flow": flowKey.ShortString()})
	}

	values := map[string]interface{}{}

	var baseFlowKey string
	if len(caller) > 0 {
		baseFlowKey = caller[0].Key.ShortString()
	} else {
		baseFlowKey = ""
	}

	ctx.Debugf("app started collecting inputs")

	flowDef, err := p.FlowRegistry.FindFlow(flowKey)
	if err != nil {
		return nil, errors.Trace(err)
	}
	for i, input := range flowDef.ResolvedInputs {
		ctx.Debugf("app sees flow depends on input %s", input.ShortName())

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
			maputil.SetValueAtPath(values, pathComponents, *arg)
		} else if provided == "" {
			var output interface{}
			var err error
			if output, err = maputil.GetValueAtPath(p.CachedFlowOutputs, pathComponents); output == nil {
				output, err = p.RunFlowForKey(p.FlowKeyCreator.CreateFlowKeyFromResolvedInput(input), []string{}, *flowDef)
				if err != nil {
					return nil, errors.Annotatef(err, "Missing value for input `%s`. Please provide a command line option or a positional argument or a flow for it`", input.ShortName())
				}
				maputil.SetValueAtPath(p.CachedFlowOutputs, pathComponents, output)
			}
			if err != nil {
				return nil, errors.Trace(err)
			}
			maputil.SetValueAtPath(values, pathComponents, output)
		} else {
			maputil.SetValueAtPath(values, pathComponents, provided)
		}

	}

	ctx.Debugf("app finished collecting inputs: %s", maputil.FlattenAsString(values))

	return values, nil
}

func (p *Application) Flows() map[string]*Flow {
	return p.FlowRegistry.Flows()
}
