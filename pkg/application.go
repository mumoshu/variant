package variant

import (
	"fmt"
	"os"
	"path"
	"strings"

	bunyan "github.com/mumoshu/logrus-bunyan-formatter"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"encoding/json"
	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/colorstring"
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
	Colorize            bool
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
		colorize := &colorstring.Colorize{
			Colors:  colorstring.DefaultColors,
			Disable: !p.Colorize,
			Reset:   true,
		}
		log.SetFormatter(&variantTextFormatter{colorize: colorize})
	} else if p.Output == "message" {
		log.SetFormatter(&MessageOnlyFormatter{})
	} else {
		log.Fatalf("Unexpected output format specified: %s", p.Output)
	}
}

func (p Application) RunTaskForKeyString(keyStr string, args []string, arguments task.Arguments, scope map[string]interface{}, asInput bool, caller ...*Task) (string, error) {
	taskKey := p.TaskNamer.FromString(fmt.Sprintf("%s.%s", p.Name, keyStr))
	return p.RunTask(taskKey, args, arguments, scope, asInput, caller...)
}

func (p Application) RunTask(taskName TaskName, args []string, arguments task.Arguments, scope map[string]interface{}, asInput bool, caller ...*Task) (string, error) {
	var ctx *log.Entry

	if len(caller) == 1 {
		ctx = log.WithFields(log.Fields{"app": p.Name, "task": taskName.ShortString(), "caller": caller[0].GetKey().ShortString()})
	} else {
		ctx = log.WithFields(log.Fields{"app": p.Name, "task": taskName.ShortString()})
	}

	ctx.Debugf("app started task %s", taskName.ShortString())

	provided := p.GetTmplOrTypedValueForConfigKey(taskName.ShortString(), "string")

	if provided != nil {
		p := fmt.Sprintf("%v", provided)
		ctx.Debugf("app skipped task %s via provided value: %s", taskName.ShortString(), p)
		ctx.Info(p)
		println(p)
		return p, nil
	}

	taskDef := p.TaskRegistry.FindTask(taskName)

	if taskDef == nil {
		return "", errors.Errorf("no task named `%s` exists", taskName.ShortString())
	}

	vars := map[string](interface{}){}
	vars["args"] = args
	vars["env"] = p.Env
	vars["cmd"] = p.CommandRelativePath

	inputs, err := p.InheritedInputValuesForTaskKey(taskName, args, arguments, scope, caller...)

	if err != nil {
		return "", errors.Wrapf(err, "%s failed running task %s", p.Name, taskName.ShortString())
	}

	for k, v := range inputs {
		vars[k] = v
	}

	{
		kv := maputil.Flatten(vars)

		s, err := jsonschemaFromInputs(taskDef.Inputs)
		if err != nil {
			return "", errors.Wrapf(err, "app failed while generating jsonschema from: %v", taskDef.Inputs)
		}
		doc := gojsonschema.NewGoLoader(kv)
		result, err := s.Validate(doc)
		if result.Valid() {
			ctx.Debugf("all the inputs are valid")
		} else {
			varsDump, err := json.MarshalIndent(vars, "", "  ")
			if err != nil {
				return "", errors.Wrapf(err, "failed marshaling error vars data %v: err", vars, err)
			}
			ctx.Errorf("one or more inputs are not valid in vars:\n%s:", varsDump)
			kvDump, err := json.MarshalIndent(kv, "", "  ")
			if err != nil {
				return "", errors.Wrapf(err, "failed marshaling error kv data %v: err", kv, err)
			}
			ctx.Errorf("one or more inputs are not valid in kv:\n%s:", kvDump)
			for _, err := range result.Errors() {
				// Err implements the ResultError interface
				ctx.Errorf("- %s", err)
			}
			return "", errors.Wrapf(err, "app failed validating inputs: %v", doc)
		}

		ctx.WithField("variables", kv).Debugf("app bound variables for task %s", taskName.ShortString())
	}

	taskTemplate := NewTaskTemplate(taskDef, vars)
	taskRunner, err := NewTaskRunner(taskDef, taskTemplate, vars)
	if err != nil {
		return "", errors.Wrapf(err, "failed to initialize task runner")
	}

	output, error := taskRunner.Run(&p, asInput, caller...)

	ctx.Debugf("app received output from task %s: %s", taskName.ShortString(), output)

	if error != nil {
		error = errors.Wrapf(error, "%s failed running task %s", p.Name, taskName.ShortString())
	}

	ctx.Debugf("app finished running task %s", taskName.ShortString())

	return output, error
}

func (p Application) InheritedInputValuesForTaskKey(taskName TaskName, args []string, arguments task.Arguments, scope map[string]interface{}, caller ...*Task) (map[string]interface{}, error) {
	result := map[string]interface{}{}

	// TODO make this parents-first instead of children-first?
	direct, err := p.DirectInputValuesForTaskKey(taskName, args, arguments, scope, caller...)

	if err != nil {
		return nil, errors.Wrapf(err, "missing input for task `%s`", taskName.ShortString())
	}

	for k, v := range direct {
		result[k] = v
	}

	parentKey, err := taskName.Parent()

	if err == nil {
		inherited, err := p.InheritedInputValuesForTaskKey(parentKey, []string{}, arguments, map[string]interface{}{}, caller...)

		if err != nil {
			return nil, errors.Wrapf(err, "AggregateInputsForParent(%s) failed", taskName.ShortString())
		}

		maputil.DeepMerge(result, inherited)
	}

	return result, nil
}

type AnyMap map[string]interface{}

func (p Application) GetTmplOrTypedValueForConfigKey(k string, tpe string) interface{} {
	ctx := log.WithFields(log.Fields{"app": p.Name, "key": k})

	if tpe == "boolean" {
		// To conform jsonschema type `boolean` to golang `bool`
		tpe = "bool"
	}

	lastIndex := strings.LastIndex(k, ".")

	ctx.Debugf("fetching %s for %s", k, tpe)

	flagKey := fmt.Sprintf("flags.%s", k)
	valueFromFlag := viper.Get(flagKey)
	ctx.Debugf("fetched %s: %v(%T)", flagKey, valueFromFlag, valueFromFlag)
	if valueFromFlag != nil && valueFromFlag != "" {
		// From flags
		if any, ok := stringToTypedValue(valueFromFlag, tpe); ok {
			return any
		}
		// From configs
		if any, ok := ensureType(valueFromFlag, tpe); ok {
			return any
		}
		ctx.Debugf("found %v(%T), but unable to convert it to %s", valueFromFlag, valueFromFlag, tpe)
	}

	ctx.Debugf("index: %d", lastIndex)

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
		ctx.Debugf("type of value fetched: expected %s, got %v", tpe, reflect.TypeOf(raw))
		if raw == nil {
			return nil
		}

		// From flags
		if s, ok := raw.(string); ok {
			return s
		}

		// From configs
		v, ok := ensureType(raw, tpe)
		if ok {
			return v
		}

		ctx.Debugf("ignoring: unexpected type of value fetched: expected %s, but got %v", tpe, reflect.TypeOf(raw))
		return nil
	}
}

func stringToTypedValue(raw interface{}, tpe string) (interface{}, bool) {
	switch s := raw.(type) {
	case string:
		switch tpe {
		case "string":
			if s == "" {
				return nil, false
			}
			return s, true
		case "integer":
			v, err := strconv.Atoi(s)
			if err != nil {
				return nil, false
			}
			return v, true
		case "bool", "boolean":
			if s == "true" {
				return true, true
			} else if s == "false" {
				return false, true
			} else {
				return nil, false
			}
		case "array":
			v := []interface{}{}
			if err := json.Unmarshal([]byte(s), &v); err != nil {
				return nil, false
			}
			return v, true

		}
	}
	return nil, false
}

func ensureType(raw interface{}, tpe string) (interface{}, bool) {
	if tpe == "string" {
		if _, ok := raw.(string); ok {
			if raw == "" {
				return nil, false
			}
			return raw, true
		}
		return nil, false
	}

	if tpe == "integer" {
		switch raw.(type) {
		case int, int64:
			return raw, true
		}
		return nil, false
	}

	if tpe == "bool" {
		switch raw.(type) {
		case bool:
			return raw, true
		}
		return nil, false
	}

	return nil, false
}

func (p Application) DirectInputValuesForTaskKey(taskName TaskName, args []string, arguments task.Arguments, scope map[string]interface{}, caller ...*Task) (map[string]interface{}, error) {
	var errs *multierror.Error

	var ctx *log.Entry

	if len(caller) == 1 {
		ctx = log.WithFields(log.Fields{"app": p.Name, "caller": caller[0].Name.ShortString(), "task": taskName.ShortString()})
	} else {
		ctx = log.WithFields(log.Fields{"app": p.Name, "task": taskName.ShortString()})
	}

	values := map[string]interface{}{}

	var baseTaskKey string
	if len(caller) > 0 {
		baseTaskKey = caller[0].GetKey().ShortString()
	} else {
		baseTaskKey = ""
	}

	ctx.Debugf("app started collecting inputs")

	currentTask := p.TaskRegistry.FindTask(taskName)
	if currentTask == nil {
		return nil, errors.Errorf("%s has no task named `%s`", p.Name, taskName)
	}

	for _, input := range currentTask.ResolvedInputs {
		ctx.Debugf("task `%s` depends on input %s", taskName, input.ShortName())

		var tmplOrStaticVal interface{}

		if i := input.ArgumentIndex; i != nil && len(args) >= *i+1 {
			ctx.Debugf("app found positional argument: args[%d]=%s", input.ArgumentIndex, args[*i])
			tmplOrStaticVal = args[*i]
		}

		if tmplOrStaticVal == nil {
			if v, err := arguments.Get(input.Name); err == nil {
				tmplOrStaticVal = v
			} else {
				errs = multierror.Append(errs, fmt.Errorf("no value for argument `%s`", input.Name))
			}
		}

		if tmplOrStaticVal == nil && input.Name != input.ShortName() {
			if v, err := arguments.Get(input.ShortName()); err == nil {
				tmplOrStaticVal = v
			} else {
				errs = multierror.Append(errs, fmt.Errorf("no value for argument `%s`", input.ShortName()))
			}
		}

		confKeyBaseTask := fmt.Sprintf("%s.%s", baseTaskKey, input.ShortName())
		if tmplOrStaticVal == nil && baseTaskKey != "" {
			tmplOrStaticVal = p.GetTmplOrTypedValueForConfigKey(confKeyBaseTask, input.TypeName())
			if tmplOrStaticVal == nil {
				errs = multierror.Append(errs, fmt.Errorf("no value for config `%s`", confKeyBaseTask))
			}
		}

		confKeyTask := fmt.Sprintf("%s.%s", taskName.ShortString(), input.ShortName())
		if tmplOrStaticVal == nil && strings.LastIndex(input.ShortName(), taskName.ShortString()) == -1 {
			tmplOrStaticVal = p.GetTmplOrTypedValueForConfigKey(confKeyTask, input.TypeName())
			if tmplOrStaticVal == nil {
				errs = multierror.Append(errs, fmt.Errorf("no value for config `%s`", confKeyTask))
			}
		}

		confKeyInput := input.ShortName()
		if tmplOrStaticVal == nil {
			tmplOrStaticVal = p.GetTmplOrTypedValueForConfigKey(confKeyInput, input.TypeName())
			if tmplOrStaticVal == nil {
				errs = multierror.Append(errs, fmt.Errorf("no value for config `%s`", confKeyInput))
			}
		}

		pathComponents := strings.Split(input.Name, ".")

		// Missed all the value sources(default, args, params, options)
		if tmplOrStaticVal == nil {
			var err error
			tmplOrStaticVal, err = maputil.GetValueAtPath(p.CachedTaskOutputs, pathComponents)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			if tmplOrStaticVal == nil {
				args := arguments.GetSubOrEmpty(input.Name)
				inTaskName := p.TaskNamer.FromResolvedInput(input)
				tmplOrStaticVal, err = p.RunTask(inTaskName, []string{}, args, map[string]interface{}{}, true, currentTask)
				if err != nil {
					ctx.Debugf("output = %v(%T)", tmplOrStaticVal, tmplOrStaticVal)
					ctx.Debugf("looking for a default value because the task %s failed", inTaskName)
					// Check if any default value is given
					if tmplOrStaticVal == nil || tmplOrStaticVal == "" {
						if input.Default != nil {
							switch input.TypeName() {
							case "string":
								tmplOrStaticVal = input.DefaultAsString()
							case "integer":
								tmplOrStaticVal = input.DefaultAsInt()
							case "boolean":
								tmplOrStaticVal = input.DefaultAsBool()
							case "array":
								v, err := input.DefaultAsArray()
								if err != nil {
									return nil, errors.Wrapf(err, "failed to parse default value as array: %v", input.Default)
								}
								tmplOrStaticVal = v
							case "object":
								v, err := input.DefaultAsObject()
								if err != nil {
									return nil, errors.Wrapf(err, "failed to parse default value as map: %v", input.Default)
								}
								tmplOrStaticVal = v
							default:
								return nil, fmt.Errorf("unsupported input type `%s` found. the type should be one of: string, integer, boolean", input.TypeName())
							}
							ctx.Debugf("got %v(%T) from default value %s(%T)", tmplOrStaticVal, tmplOrStaticVal, input.Default, input.Default)
						} else if input.Name == "env" {
							tmplOrStaticVal = ""
						} else {
							errs = multierror.Append(errs, fmt.Errorf("no default value defined for input `%s`", input.Name))
						}
					}

					if tmplOrStaticVal == nil {
						// No default value given
						runTaskErr := errors.Wrapf(err, "unable to run task `%s`", inTaskName)
						errs = multierror.Append(errs, runTaskErr)
						errs.ErrorFormat = func(es []error) string {
							points := make([]string, len(es))
							for i, err := range es {
								points[i] = fmt.Sprintf("%d. %s", i+1, err)
							}
							return fmt.Sprintf("all the input sources failed (details follow)\n%s", strings.Join(points, "\n"))
						}
						return nil, errors.WithStack(errs)
					}
				} else {
					maputil.SetValueAtPath(p.CachedTaskOutputs, pathComponents, tmplOrStaticVal)
				}
			}
		}

		// Now that the tmplOrStaticVal exists, render add type it
		log.Debugf("tmplOrStaticVal=%v", tmplOrStaticVal)
		if tmplOrStaticVal != nil {
			var renderedValue string
			expr, ok := tmplOrStaticVal.(string)
			if ok {
				taskTemplate := NewTaskTemplate(currentTask, scope)
				log.Debugf("rendering %s", expr)
				r, err := taskTemplate.Render(expr, input.Name)
				if err != nil {
					return nil, errors.Wrap(err, "failed to render task template")
				}
				renderedValue = r
				log.Debugf("converting type of %v", renderedValue)
				tmplOrStaticVal, err = parseSupportedValueFromString(renderedValue, input.TypeName())
				if err != nil {
					return nil, err
				}
				log.Debugf("value after type conversion=%v", tmplOrStaticVal)
			}
		} else {
			// the dependent task succeeded with no output
		}

		maputil.SetValueAtPath(values, pathComponents, tmplOrStaticVal)
	}

	ctx.WithField("values", values).Debugf("app finished collecting inputs")

	return values, nil
}

func parseSupportedValueFromString(renderedValue string, typeName string) (interface{}, error) {
	switch typeName {
	case "string":
		log.Debugf("string=%v", renderedValue)
		return renderedValue, nil
	case "integer":
		log.Debugf("integer=%v", renderedValue)
		value, err := strconv.Atoi(renderedValue)
		if err != nil {
			return nil, errors.Wrapf(err, "%v can't be casted to integer", renderedValue)
		}
		return value, nil
	case "boolean":
		log.Debugf("boolean=%v", renderedValue)
		switch renderedValue {
		case "true":
			return true, nil
		case "false":
			return false, nil
		default:
			return nil, fmt.Errorf("%v can't be parsed as boolean", renderedValue)
		}
	case "array", "object":
		log.Debugf("converting this to either array or object=%v", renderedValue)
		var dst interface{}
		if err := json.Unmarshal([]byte(renderedValue), &dst); err != nil {
			return nil, errors.Wrapf(err, "failed converting: failed to parse %s as json", renderedValue)
		}
		return dst, nil
	default:
		log.Debugf("foobar")
		return nil, fmt.Errorf("unsupported input type `%s` found. the type should be one of: string, integer, boolean", typeName)
	}
}

func (p *Application) Tasks() map[string]*Task {
	return p.TaskRegistry.Tasks()
}

func jsonschemaFromInputs(inputs []*InputConfig) (*gojsonschema.Schema, error) {
	required := []string{}
	props := map[string]map[string]interface{}{}
	for _, input := range inputs {
		name := strings.Replace(input.Name, "-", "_", -1)
		props[name] = input.JSONSchema()

		if input.Required() {
			required = append(required, name)
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
