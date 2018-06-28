package variant

import (
	"fmt"
	"os"
	"path"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/juju/errors"
	bunyan "github.com/mumoshu/logrus-bunyan-formatter"
	"github.com/spf13/viper"

	"github.com/mumoshu/variant/pkg/api/step"
	"github.com/mumoshu/variant/pkg/api/task"
	"github.com/mumoshu/variant/pkg/util/maputil"
	"github.com/xeipuuv/gojsonschema"
	"reflect"
	"strconv"
)

type Application struct {
	Name                string
	CommandRelativePath string
	CachedTaskOutputs   map[string]interface{}
	ConfigFile          string
	Verbose             bool
	Output              string
	Env                 string
	TaskRegistry        *TaskRegistry
	InputResolver       InputResolver
	TaskNamer           *TaskNamer
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

func (p Application) RunTaskForKeyString(keyStr string, args []string, provided task.ProvidedInputs, caller ...step.Caller) (string, error) {
	taskKey := p.TaskNamer.FromString(fmt.Sprintf("%s.%s", p.Name, keyStr))
	return p.RunTaskForKey(taskKey, args, provided, caller...)
}

func (p Application) RunTaskForKey(taskKey step.Key, args []string, providedInputs task.ProvidedInputs, caller ...step.Caller) (string, error) {
	var ctx *log.Entry

	if len(caller) == 1 {
		ctx = log.WithFields(log.Fields{"task": taskKey.ShortString(), "caller": caller[0].GetKey().ShortString()})
	} else {
		ctx = log.WithFields(log.Fields{"task": taskKey.ShortString()})
	}

	ctx.Debugf("app started task %s", taskKey.ShortString())

	provided := p.GetValueForConfigKey(taskKey.ShortString())

	if provided != nil {
		p := fmt.Sprintf("%v", provided)
		ctx.Debugf("app skipped task %s via provided value: %s", taskKey.ShortString(), p)
		ctx.Info(p)
		println(p)
		return p, nil
	}

	taskDef, err := p.TaskRegistry.FindTask(taskKey)

	if err != nil {
		return "", errors.Annotatef(err, "app failed finding task %s", taskKey.ShortString())
	}

	vars := map[string](interface{}){}
	vars["args"] = args
	vars["env"] = p.Env
	vars["cmd"] = p.CommandRelativePath

	inputs, err := p.InheritedInputValuesForTaskKey(taskKey, args, providedInputs, caller...)

	if err != nil {
		return "", errors.Annotatef(err, "app failed running task %s", taskKey.ShortString())
	}

	for k, v := range inputs {
		vars[k] = v
	}

	task := &BoundTask{
		Vars: vars,
		Task: *taskDef,
	}

	kv := maputil.Flatten(vars)

	s, err := jsonschemaFromInputs(taskDef.Inputs)
	if err != nil {
		return "", errors.Annotatef(err, "app failed while generating jsonschema from: %v", taskDef.Inputs)
	}
	doc := gojsonschema.NewGoLoader(kv)
	result, err := s.Validate(doc)
	if result.Valid() {
		log.Debugf("all the inputs are valid")
	} else {
		log.Errorf("one or more inputs are not valid in %v:", kv)
		for _, err := range result.Errors() {
			// Err implements the ResultError interface
			log.Errorf("- %s", err)
		}
		return "", errors.Annotatef(err, "app failed validating inputs: %v", doc)
	}

	ctx.WithField("variables", kv).Debugf("app bound variables for task %s", taskKey.ShortString())

	output, error := task.Run(&p, caller...)

	ctx.Debugf("app received output from task %s: %s", taskKey.ShortString(), output)

	if error != nil {
		error = errors.Annotatef(error, "app failed running task %s", taskKey.ShortString())
	}

	ctx.Debugf("app finished running task %s", taskKey.ShortString())

	return output, error
}

func (p Application) InheritedInputValuesForTaskKey(taskKey step.Key, args []string, provided task.ProvidedInputs, caller ...step.Caller) (map[string]interface{}, error) {
	result := map[string]interface{}{}

	// TODO make this parents-first instead of children-first?
	direct, err := p.DirectInputValuesForTaskKey(taskKey, args, provided, caller...)

	if err != nil {
		return nil, errors.Annotatef(err, "One or more inputs for task %s failed", taskKey.ShortString())
	}

	for k, v := range direct {
		result[k] = v
	}

	parentKey, err := taskKey.Parent()

	if err == nil {
		inherited, err := p.InheritedInputValuesForTaskKey(parentKey, []string{}, provided, caller...)

		if err != nil {
			return nil, errors.Annotatef(err, "AggregateInputsForParent(%s) failed", taskKey.ShortString())
		}

		maputil.DeepMerge(result, inherited)
	}

	return result, nil
}

type AnyMap map[string]interface{}

func (p Application) GetValueForConfigKey(k string) interface{} {
	ctx := log.WithFields(log.Fields{"key": k})

	lastIndex := strings.LastIndex(k, ".")

	valueFromFlag := viper.Get(fmt.Sprintf("flags.%s", k))
	if valueFromFlag != nil {
		if str, ok := valueFromFlag.(string); ok && str == "" {
			return nil
		}
		ctx.Debugf("GetValueForConfigKey(%s): %v", k, valueFromFlag)
		return valueFromFlag
	}

	if lastIndex != -1 {
		a := []rune(k)
		k1 := string(a[:lastIndex])
		k2 := string(a[lastIndex+1:])

		ctx.Debugf("viper.Get(%v): %v", k1, viper.Get(k1))

		if viper.Get(k1) != nil {

			values := viper.Sub(k1)

			ctx.Debugf("app fetched %s: %v", k1, values)

			var provided interface{}

			if values != nil && values.Get(k2) != nil {
				provided = values.Get(k2)
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
		if raw == nil {
			return nil
		}
		switch raw.(type) {
		case string, int, int64, bool:
			return raw
		default:
			panic(fmt.Sprintf("unexpected type of value fetched: %v", reflect.TypeOf(raw)))
		}
	}
}

func (p Application) DirectInputValuesForTaskKey(taskKey step.Key, args []string, provided task.ProvidedInputs, caller ...step.Caller) (map[string]interface{}, error) {
	var ctx *log.Entry

	if len(caller) == 1 {
		ctx = log.WithFields(log.Fields{"caller": caller[0].GetKey().ShortString(), "task": taskKey.ShortString()})
	} else {
		ctx = log.WithFields(log.Fields{"task": taskKey.ShortString()})
	}

	values := map[string]interface{}{}

	var baseTaskKey string
	if len(caller) > 0 {
		baseTaskKey = caller[0].GetKey().ShortString()
	} else {
		baseTaskKey = ""
	}

	ctx.Debugf("app started collecting inputs")

	taskDef, err := p.TaskRegistry.FindTask(taskKey)
	if err != nil {
		return nil, errors.Trace(err)
	}
	for _, input := range taskDef.ResolvedInputs {
		ctx.Debugf("app sees task depends on input %s", input.ShortName())

		var positional interface{}
		if i := input.ArgumentIndex; i != nil && len(args) >= *i+1 {
			ctx.Debugf("app found positional argument: args[%d]=%s", input.ArgumentIndex, args[*i])
			positional = args[*i]
		}

		var nullableValue interface{}

		if v, err := provided.Get(input.Name); err == nil {
			nullableValue = v
		}

		if nullableValue == nil && baseTaskKey != "" {
			nullableValue = p.GetValueForConfigKey(fmt.Sprintf("%s.%s", baseTaskKey, input.ShortName()))
		}

		if nullableValue == nil && strings.LastIndex(input.ShortName(), taskKey.ShortString()) == -1 {
			nullableValue = p.GetValueForConfigKey(fmt.Sprintf("%s.%s", taskKey.ShortString(), input.ShortName()))
		}

		if nullableValue == nil {
			nullableValue = p.GetValueForConfigKey(input.ShortName())
		}
		log.Debugf("nullableValue=%v", nullableValue)
		if nullableValue != nil {
			log.Debugf("converting type of %v", nullableValue)
			switch input.TypeName() {
			case "string":
				log.Debugf("string=%v", nullableValue)
				nullableValue = nullableValue
			case "integer":
				log.Debugf("integer=%v", nullableValue)
				s := nullableValue.(string)
				nullableValue, err = strconv.Atoi(s)
				if err != nil {
					return nil, errors.Annotatef(err, "%v can't be casted to integer", s)
				}
			case "boolean":
				log.Debugf("boolean=%v", nullableValue)
				s := nullableValue.(string)
				switch s {
				case "true":
					nullableValue = true
				case "false":
					nullableValue = false
				default:
					return nil, fmt.Errorf("%v can't be parsed as boolean", s)
				}
			default:
				log.Debugf("foobar")
				return nil, fmt.Errorf("unsupported input type `%s` found. the type should be one of: string, integer, boolean", input.TypeName())
			}
			log.Debugf("value after type conversion=%v", nullableValue)
		}

		if nullableValue == nil && input.Default != nil {
			switch input.TypeName() {
			case "string":
				nullableValue = input.DefaultAsString()
			case "integer":
				nullableValue = input.DefaultAsInt()
			case "boolean":
				nullableValue = input.DefaultAsBool()
			default:
				return nil, fmt.Errorf("unsupported input type `%s` found. the type should be one of: string, integer, boolean", input.TypeName())
			}
		}

		if nullableValue == nil && input.Name == "env" {
			nullableValue = ""
		}

		pathComponents := strings.Split(input.Name, ".")

		if positional != nil {
			maputil.SetValueAtPath(values, pathComponents, positional)
		} else if nullableValue == nil {
			var output interface{}
			var err error
			if output, err = maputil.GetValueAtPath(p.CachedTaskOutputs, pathComponents); output == nil {
				output, err = p.RunTaskForKey(p.TaskNamer.FromResolvedInput(input), []string{}, task.NewProvidedInputs(), *taskDef)
				if err != nil {
					return nil, errors.Annotatef(err, "Missing value for input `%s`. Please provide a command line option or a positional argument or a task for it`", input.ShortName())
				}
				maputil.SetValueAtPath(p.CachedTaskOutputs, pathComponents, output)
			}
			if err != nil {
				return nil, errors.Trace(err)
			}
			maputil.SetValueAtPath(values, pathComponents, output)
		} else {
			maputil.SetValueAtPath(values, pathComponents, nullableValue)
		}

	}

	ctx.WithField("values", values).Debugf("app finished collecting inputs")

	return values, nil
}

func (p *Application) Tasks() map[string]*Task {
	return p.TaskRegistry.Tasks()
}

func jsonschemaFromInputs(inputs []*InputConfig) (*gojsonschema.Schema, error) {
	required := []string{}
	props := map[string]map[string]interface{}{}
	for _, input := range inputs {
		props[input.Name] = input.JSONSchema()

		if input.Required() {
			required = append(required, input.Name)
		}
	}
	goschema := map[string]interface{}{
		"type":       "object",
		"properties": props,
		"required":   required,
	}
	schemaLoader := gojsonschema.NewGoLoader(goschema)
	return gojsonschema.NewSchema(schemaLoader)
}
