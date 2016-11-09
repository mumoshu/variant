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

	"../api/flow"
	"../api/step"
	"../util/maputil"
	"reflect"
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
	LogToStderr         bool
}

func (p Application) UpdateLoggingConfiguration() {
	if p.Verbose {
		log.SetLevel(log.DebugLevel)
	}

	if p.LogToStderr {
		log.SetOutput(os.Stderr)
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

func (p Application) RunFlowForKeyString(keyStr string, args []string, provided flow.ProvidedInputs, caller ...step.Caller) (string, error) {
	flowKey := p.FlowKeyCreator.CreateFlowKey(fmt.Sprintf("%s.%s", p.Name, keyStr))
	return p.RunFlowForKey(flowKey, args, provided, caller...)
}

func (p Application) RunFlowForKey(flowKey step.Key, args []string, providedInputs flow.ProvidedInputs, caller ...step.Caller) (string, error) {
	var ctx *log.Entry

	if len(caller) == 1 {
		ctx = log.WithFields(log.Fields{"flow": flowKey.ShortString(), "caller": caller[0].GetKey().ShortString()})
	} else {
		ctx = log.WithFields(log.Fields{"flow": flowKey.ShortString()})
	}

	ctx.Debugf("app started flow %s", flowKey.ShortString())

	provided := p.GetValueForConfigKey(flowKey.ShortString())

	if provided != nil {
		p := *provided
		ctx.Debugf("app skipped flow %s via provided value: %s", flowKey.ShortString(), p)
		ctx.Info(p)
		println(p)
		return p, nil
	}

	flowDef, err := p.FlowRegistry.FindFlow(flowKey)

	if err != nil {
		return "", errors.Annotatef(err, "app failed finding flow %s", flowKey.ShortString())
	}

	vars := map[string](interface{}){}
	vars["args"] = args
	vars["env"] = p.Env
	vars["cmd"] = p.CommandRelativePath

	inputs, err := p.InheritedInputValuesForFlowKey(flowKey, args, providedInputs, caller...)

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

	kv := maputil.Flatten(vars)

	ctx.WithField("variables", kv).Debugf("app bound variables for flow %s", flowKey.ShortString())

	output, error := flow.Run(&p, caller...)

	ctx.Debugf("app received output from flow %s: %s", flowKey.ShortString(), output)

	if error != nil {
		error = errors.Annotatef(error, "app failed running flow %s", flowKey.ShortString())
	}

	ctx.Debugf("app finished running flow %s", flowKey.ShortString())

	return output, error
}

func (p Application) InheritedInputValuesForFlowKey(flowKey step.Key, args []string, provided flow.ProvidedInputs, caller ...step.Caller) (map[string]interface{}, error) {
	result := map[string]interface{}{}

	// TODO make this parents-first instead of children-first?
	direct, err := p.DirectInputValuesForFlowKey(flowKey, args, provided, caller...)

	if err != nil {
		return nil, errors.Annotatef(err, "One or more inputs for flow %s failed", flowKey.ShortString())
	}

	for k, v := range direct {
		result[k] = v
	}

	parentKey, err := flowKey.Parent()

	if err == nil {
		inherited, err := p.InheritedInputValuesForFlowKey(parentKey, []string{}, provided, caller...)

		if err != nil {
			return nil, errors.Annotatef(err, "AggregateInputsForParent(%s) failed", flowKey.ShortString())
		}

		maputil.DeepMerge(result, inherited)
	}

	return result, nil
}

type AnyMap map[string]interface{}

func (p Application) GetValueForConfigKey(k string) *string {
	ctx := log.WithFields(log.Fields{"key": k})

	lastIndex := strings.LastIndex(k, ".")

	valueFromFlag := viper.GetString(fmt.Sprintf("flags.%s", k))

	if valueFromFlag != "" {
		return &valueFromFlag
	}

	if lastIndex != -1 {
		a := []rune(k)
		k1 := string(a[:lastIndex])
		k2 := string(a[lastIndex+1:])

		ctx.Debugf("viper.Get(%v): %v", k1, viper.Get(k1))

		if viper.Get(k1) != nil {

			values := viper.Sub(k1)

			ctx.Debugf("app fetched %s: %v", k1, values)

			var provided *string

			if values != nil && values.Get(k2) != nil {
				str := values.GetString(k2)
				provided = &str
			} else {
				provided = nil
			}

			ctx.Debugf("app fetched %s[%s]: %s", k1, k2, provided)

			if provided != nil {
				return provided
			}
		}
		return nil
	} else {
		raw := viper.Get(k)
		ctx.Debugf("app fetched raw value for key %s: %v", k, raw)
		ctx.Debugf("type of value fetched: %v", reflect.TypeOf(raw))
		if str, ok := raw.(string); ok {
			return &str
		} else if raw == nil {
			return nil
		} else {
			panic(fmt.Sprintf("unexpected type of value fetched: %v", reflect.TypeOf(raw)))
		}
	}
}

func (p Application) DirectInputValuesForFlowKey(flowKey step.Key, args []string, provided flow.ProvidedInputs, caller ...step.Caller) (map[string]interface{}, error) {
	var ctx *log.Entry

	if len(caller) == 1 {
		ctx = log.WithFields(log.Fields{"caller": caller[0].GetKey().ShortString(), "flow": flowKey.ShortString()})
	} else {
		ctx = log.WithFields(log.Fields{"flow": flowKey.ShortString()})
	}

	values := map[string]interface{}{}

	var baseFlowKey string
	if len(caller) > 0 {
		baseFlowKey = caller[0].GetKey().ShortString()
	} else {
		baseFlowKey = ""
	}

	ctx.Debugf("app started collecting inputs")

	flowDef, err := p.FlowRegistry.FindFlow(flowKey)
	if err != nil {
		return nil, errors.Trace(err)
	}
	for _, input := range flowDef.ResolvedInputs {
		ctx.Debugf("app sees flow depends on input %s", input.ShortName())

		var positional *string
		if i := input.ArgumentIndex; i != nil && len(args) >= *i+1 {
			ctx.Debugf("app found positional argument: args[%d]=%s", input.ArgumentIndex, args[*i])
			positional = &args[*i]
		}

		var nullableValue *string

		if v, err := provided.Get(input.Name); err == nil {
			nullableValue = &v
		}

		if nullableValue == nil && baseFlowKey != "" {
			nullableValue = p.GetValueForConfigKey(fmt.Sprintf("%s.%s", baseFlowKey, input.ShortName()))
		}

		if nullableValue == nil && strings.LastIndex(input.ShortName(), flowKey.ShortString()) == -1 {
			nullableValue = p.GetValueForConfigKey(fmt.Sprintf("%s.%s", flowKey.ShortString(), input.ShortName()))
		}

		if nullableValue == nil {
			nullableValue = p.GetValueForConfigKey(input.ShortName())
		}

		pathComponents := strings.Split(input.Name, ".")

		if positional != nil {
			maputil.SetValueAtPath(values, pathComponents, *positional)
		} else if nullableValue == nil {
			var output interface{}
			var err error
			if output, err = maputil.GetValueAtPath(p.CachedFlowOutputs, pathComponents); output == nil {
				output, err = p.RunFlowForKey(p.FlowKeyCreator.CreateFlowKeyFromResolvedInput(input), []string{}, flow.NewProvidedInputs(), *flowDef)
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
			maputil.SetValueAtPath(values, pathComponents, *nullableValue)
		}

	}

	ctx.WithField("values", values).Debugf("app finished collecting inputs")

	return values, nil
}

func (p *Application) Flows() map[string]*Flow {
	return p.FlowRegistry.Flows()
}
