package variant

import (
	"fmt"
	"github.com/mumoshu/variant/pkg/util/fileutil"
	"os"
	"path/filepath"
	"strings"

	bunyan "github.com/mumoshu/logrus-bunyan-formatter"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"encoding/json"
	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/colorstring"
	"github.com/mumoshu/variant/pkg/api/task"
	"github.com/mumoshu/variant/pkg/get"
	"github.com/mumoshu/variant/pkg/util/maputil"
	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v2"
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
	NoColorize          bool
	Env                 string
	TaskRegistry        *TaskRegistry
	InputResolver       InputResolver
	TaskNamer           *TaskNamer
	LogToStderr         bool

	LogLevel      string
	LogColorPanic string
	LogColorFatal string
	LogColorError string
	LogColorWarn  string
	LogColorInfo  string
	LogColorDebug string
	LogColorTrace string

	LastRun string

	LastOutputs map[string]string

	Viper *viper.Viper

	Log *logrus.Logger

	ConfigContexts []string
	ConfigDirs     []string
	CommandName    string
}

func (p *Application) Color() bool {
	return p.Colorize && !p.NoColorize
}

func (p *Application) setGlobalParams() {
	p.Verbose = p.Viper.GetBool("verbose")
	p.Colorize = p.Viper.GetBool("color") && !p.Viper.GetBool("no-color")
	p.LogToStderr = p.Viper.GetBool("logtostderr")
	p.Output = p.Viper.GetString("output")
	p.ConfigFile = p.Viper.GetString("config-file")

	p.LogLevel = p.Viper.GetString("log-level")
	p.LogColorPanic = p.Viper.GetString("log-color-panic")
	p.LogColorFatal = p.Viper.GetString("log-color-fatal")
	p.LogColorError = p.Viper.GetString("log-color-error")
	p.LogColorWarn = p.Viper.GetString("log-color-warn")
	p.LogColorInfo = p.Viper.GetString("log-color-info")
	p.LogColorDebug = p.Viper.GetString("log-color-debug")
	p.LogColorTrace = p.Viper.GetString("log-color-trace")

	// Handle multiple config contexts set
	configContexts := p.Viper.GetStringSlice("config-context")
	for _, c := range configContexts {
		if c != "[]" {
			s := strings.Split(c, ",")
			p.ConfigContexts = append(p.ConfigContexts, s...)
		}
	}
	// Set default config contexts
	p.ConfigContexts = append([]string{p.CommandName}, p.ConfigContexts...)

	// Handle multiple config dirs set
	configDirs := p.Viper.GetStringSlice("config-dir")
	for _, c := range configDirs {
		if c != "[]" {
			s := strings.Split(c, ",")
			p.ConfigDirs = append(p.ConfigDirs, s...)
		}
	}
	// Set default config contexts
	p.ConfigDirs = maputil.SliceUnique(append([]string{filepath.Dir(p.CommandRelativePath), "."}, p.ConfigDirs...))
}

func (p *Application) loadContextConfigs() {
	var contexts []string
	for _, c := range p.ConfigContexts {
		contexts = append(contexts, c)
	}
	for i, c1 := range p.ConfigContexts {
		prefix := c1
		for j, c2 := range p.ConfigContexts {
			if j > i {
				prefix = strings.Join([]string{prefix, c2}, ".")
				combo := strings.Join([]string{c1, c2}, ".")
				if combo != prefix && c1 != c2 {
					contexts = append(contexts, combo)
				}
				contexts = append(contexts, prefix)
			}
		}
	}
	for _, d := range p.ConfigDirs {
		for _, c := range contexts {
			p.loadConfig(filepath.Join(d, c))
		}
	}
}

func (p *Application) loadConfigFile(fileName string) error {
	msg := fmt.Sprintf("loading config file %s...", fileName)
	if fileutil.Exists(fileName) {
		p.Viper.SetConfigFile(fileName)

		// See "How to merge two config files" https://github.com/spf13/viper/issues/181
		if err := p.Viper.MergeInConfig(); err != nil {
			p.Log.Errorf("%serror", fileName)
			return err
		}
		p.Log.Infof("%s done", msg)
	} else {
		p.Log.Debugf("%s missing", msg)
	}
	return nil
}

func (p *Application) loadConfig(configName string) error {
	return p.loadConfigFile(fmt.Sprintf("%s.yaml", configName))
}

func (p *Application) UpdateLoggingConfiguration() error {
	if p.Verbose {
		p.Log.SetLevel(logrus.DebugLevel)
	} else {
		ls := p.Viper.Get("log-level").(string)
		l, err := logrus.ParseLevel(ls)
		if err != nil {
			return fmt.Errorf("log level is not specified properly: \"%s\", use one of the: \"panic\", \"fatal\", \"error\", \"warn\", \"info\", \"debug\", \"trace\"", ls)
		}
		p.Log.SetLevel(l)
	}

	if p.LogToStderr {
		p.Log.SetOutput(os.Stderr)
	}

	commandName := filepath.Base(os.Args[0])
	if p.Output == "bunyan" {
		p.Log.SetFormatter(&bunyan.Formatter{Name: commandName})
	} else if p.Output == "json" {
		p.Log.SetFormatter(&logrus.JSONFormatter{})
	} else if p.Output == "text" {
		colorize := &colorstring.Colorize{
			Colors:  colorstring.DefaultColors,
			Disable: !p.Color(),
			Reset:   true,
		}

		p.Log.SetFormatter(&variantTextFormatter{
			colorize: colorize,
			colors: map[logrus.Level]string{
				logrus.PanicLevel: p.LogColorPanic,
				logrus.FatalLevel: p.LogColorFatal,
				logrus.ErrorLevel: p.LogColorError,
				logrus.WarnLevel:  p.LogColorWarn,
				logrus.InfoLevel:  p.LogColorInfo,
				logrus.DebugLevel: p.LogColorDebug,
				logrus.TraceLevel: p.LogColorTrace,
			},
		})
	} else if p.Output == "message" {
		p.Log.SetFormatter(&MessageOnlyFormatter{})
	} else {
		return fmt.Errorf("unexpected output format specified: %s", p.Output)
	}
	return nil
}

func (p *Application) RunTaskForKeyString(keyStr string, args []string, arguments task.Arguments, scope map[string]interface{}, asInput bool, caller ...*Task) (string, error) {
	taskKey := p.TaskNamer.FromString(fmt.Sprintf("%s.%s", p.Name, keyStr))
	return p.RunTask(taskKey, args, arguments, scope, asInput, caller...)
}

func (p *Application) Run(taskName TaskName, args []string) error {
	p.LastRun = taskName.ShortString()

	errMsg, err := p.RunTask(taskName, args, task.NewArguments(), map[string]interface{}{}, false)

	if err != nil {
		return CommandError{error: err, TaskName: taskName, Cause: errMsg}
	}
	return nil
}

func (p *Application) RunTask(taskName TaskName, args []string, arguments task.Arguments, scope map[string]interface{}, asInput bool, caller ...*Task) (string, error) {
	var ctx *logrus.Entry

	if len(caller) == 1 {
		ctx = p.Log.WithFields(logrus.Fields{"app": p.Name, "task": taskName.ShortString(), "caller": caller[0].GetKey().ShortString()})
	} else {
		ctx = p.Log.WithFields(logrus.Fields{"app": p.Name, "task": taskName.ShortString()})
	}

	ctx.Debugf("app started task %s", taskName.ShortString())

	//provided := p.GetTmplOrTypedValueForConfigKey(taskName.ShortString(), "string")
	//
	//if provided != nil {
	//	p := fmt.Sprintf("%v", provided)
	//	ctx.Debugf("app skipped task %s via provided value: %s", taskName.ShortString(), p)
	//	ctx.Info(p)
	//	println(p)
	//	return p, nil
	//}

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

		s, err := p.jsonschemaFromInputs(taskDef.Inputs)
		if err != nil {
			ins := []InputConfig{}
			for _, v := range taskDef.Inputs {
				if v != nil {
					ins = append(ins, *v)
				}
			}
			return "", errors.Wrapf(err, "app failed while generating jsonschema from:\n%+v", ins)
		}
		doc := gojsonschema.NewGoLoader(vars)
		result, err := s.Validate(doc)
		if err != nil {
			return "", errors.Wrapf(err, "fix your parameter value")
		}
		if result.Valid() {
			ctx.Debugf("all the inputs are valid")
		} else {
			varsDump, err := json.MarshalIndent(vars, "", "  ")
			if err != nil {
				return "", errors.Wrapf(err, "failed marshaling error vars data %v: %v", vars, err)
			}
			ctx.Errorf("one or more inputs are not valid in vars:\n%+v:", vars)
			ctx.Errorf("one or more inputs are not valid in varsDumo:\n%s:", varsDump)
			kvDump, err := json.MarshalIndent(kv, "", "  ")
			if err != nil {
				return "", errors.Wrapf(err, "failed marshaling error kv data %v: %v", kv, err)
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

	output, error := taskRunner.Run(p, asInput, caller...)

	ctx.Debugf("app received output from task %s: %s", taskName.ShortString(), output)

	if error != nil {
		error = errors.Wrapf(error, "%s failed running task %s", p.Name, taskName.ShortString())
	}

	if p.LastOutputs == nil {
		p.LastOutputs = map[string]string{}
	}
	p.LastOutputs[taskName.ShortString()] = output

	ctx.Debugf("app finished running task %s", taskName.ShortString())

	return output, error
}

func (p Application) InheritedInputValuesForTaskKey(taskName TaskName, args []string, arguments task.Arguments, scope map[string]interface{}, caller ...*Task) (map[string]interface{}, error) {
	result := map[string]interface{}{}

	for k, _ := range taskName.Components {
		direct, err := p.DirectInputValuesForTaskKey(TaskName{Components: taskName.Components[:k+1]}, args, arguments, scope, caller...)
		if err != nil {
			return nil, errors.Wrapf(err, "missing input for task `%s`", taskName.ShortString())
		}
		maputil.DeepMerge(result, direct)
	}

	return result, nil
}

type AnyMap map[string]interface{}

func sourceToObject(v interface{}) (map[string]interface{}, error) {
	if s, ok := v.(string); ok && s != "" {
		dst := map[string]interface{}{}
		if err := get.Unmarshal(s, &dst); err != nil {
			return nil, err
		}
		r, err := maputil.RecursivelyStringifyKeys(dst)
		if err != nil {
			return nil, err
		}
		return r, nil
	}
	return nil, nil
}

func (p Application) GetTmplOrTypedValueForConfigKey(k string, tpe string, bindEnvVars bool) interface{} {
	ctx := p.Log.WithFields(logrus.Fields{"app": p.Name, "key": k})

	convert := func(v interface{}) (interface{}, bool) {
		// Import a file and parse the content into a value
		if tpe == "object" {
			r, err := sourceToObject(v)
			if err != nil {
				ctx.Errorf("%v", err)
				return nil, false
			}
			if r != nil {
				return r, true
			}
		}

		// From flags
		if any, ok := stringToTypedValue(v, tpe); ok {
			return any, true
		}
		// From configs
		if any, ok := ensureType(v, tpe); ok {
			return any, true
		}
		ctx.Debugf("ignoring value: found %v(%T), but unable to convert it to %s", v, v, tpe)

		return nil, false
	}

	if tpe == "boolean" {
		// To conform jsonschema type `boolean` to golang `bool`
		tpe = "bool"
	}

	lastIndex := strings.LastIndex(k, ".")

	ctx.Debugf("fetching %s for %s", k, tpe)

	flagKey := fmt.Sprintf("flags.%s", k)
	valueFromFlag := p.Viper.Get(flagKey)
	ctx.Debugf("fetched %s: %v(%T)", flagKey, valueFromFlag, valueFromFlag)
	if valueFromFlag != nil && valueFromFlag != "" {
		if any, ok := convert(valueFromFlag); ok {
			return any
		}
	}

	ctx.Debugf("index: %d", lastIndex)

	var value interface{}

	if lastIndex != -1 {
		a := []rune(k)
		parentKey := string(a[:lastIndex])
		childKey := string(a[lastIndex+1:])

		parentValue := viper.Get(parentKey)
		ctx.Debugf("viper.Get(%v): %v", parentKey, parentValue)

		if parentValue != nil {

			values := p.Viper.Sub(parentKey)

			ctx.Debugf("app fetched %s: %v", parentKey, values)

			var childValue interface{}

			if values != nil {
				childValue = values.Get(childKey)
				ctx.Debugf("app fetched %s[%s]: %v(%T)", parentKey, childKey, childValue, childValue)
				value = childValue
			}
		}
	} else {
		if bindEnvVars {
			// Bind parameter to the environment variable without a prefix ("PARAM1" vs "VARIANT_FLAGS_PARAM1").
			p.Viper.BindEnv(k, strings.ToUpper(k))
		}
		raw := p.Viper.Get(k)
		ctx.Debugf("app fetched raw value for key %s: %v", k, raw)
		ctx.Debugf("type of value fetched: expected %s, got %v", tpe, reflect.TypeOf(raw))
		if raw == nil {
			return nil
		}

		value = raw
	}

	if value == "" {
		return value
	} else if value != nil {
		if v, ok := convert(value); ok {
			return v
		}
	}

	return nil
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

	if tpe == "object" {
		switch raw.(type) {
		case map[string]interface{}:
			return raw, true
		}
		return nil, false
	}

	if tpe == "array" {
		switch r := raw.(type) {
		case []interface{}:
			a := []interface{}{}
			for i := range r {
				m, err := maputil.RecursivelyStringifyKeys(r[i])
				if err != nil {
					fmt.Fprintf(os.Stderr, "unexpected error while processing array: %v", err)
					return nil, false
				}
				a = append(a, m)
			}
			return a, true
		}
		return nil, false
	}

	return nil, false
}

func (p Application) DirectInputValuesForTaskKey(taskName TaskName, args []string, arguments task.Arguments, scope map[string]interface{}, caller ...*Task) (map[string]interface{}, error) {
	var errs *multierror.Error

	var ctx *logrus.Entry

	if len(caller) == 1 {
		ctx = p.Log.WithFields(logrus.Fields{"app": p.Name, "caller": caller[0].Name.ShortString(), "task": taskName.ShortString()})
	} else {
		ctx = p.Log.WithFields(logrus.Fields{"app": p.Name, "task": taskName.ShortString()})
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
			if str, err := arguments.GetString(input.Name); err == nil && str != "" {
				tmplOrStaticVal, err = p.parseSupportedValueFromString(str, input.TypeName())
				if err != nil {
					return nil, err
				}
			} else {
				errs = multierror.Append(errs, fmt.Errorf("no value for argument `%s`", input.Name))
			}
		}

		if tmplOrStaticVal == nil && input.Name != input.ShortName() {
			if str, err := arguments.GetString(input.ShortName()); err == nil && str != "" {
				tmplOrStaticVal, err = p.parseSupportedValueFromString(str, input.TypeName())
				if err != nil {
					return nil, err
				}
			} else {
				errs = multierror.Append(errs, fmt.Errorf("no value for argument `%s`", input.ShortName()))
			}
		}

		confKeyBaseTask := fmt.Sprintf("%s.%s", baseTaskKey, input.ShortName())
		if tmplOrStaticVal == nil && baseTaskKey != "" {
			tmplOrStaticVal = p.GetTmplOrTypedValueForConfigKey(confKeyBaseTask, input.TypeName(), currentTask.TaskDef.BindParamsFromEnv)
			if tmplOrStaticVal == nil {
				errs = multierror.Append(errs, fmt.Errorf("no value for config `%s`", confKeyBaseTask))
			}
		}

		confKeyTask := fmt.Sprintf("%s.%s", taskName.ShortString(), input.ShortName())
		if tmplOrStaticVal == nil && strings.LastIndex(input.ShortName(), taskName.ShortString()) == -1 {
			tmplOrStaticVal = p.GetTmplOrTypedValueForConfigKey(confKeyTask, input.TypeName(), currentTask.TaskDef.BindParamsFromEnv)
			if tmplOrStaticVal == nil {
				errs = multierror.Append(errs, fmt.Errorf("no value for config `%s`", confKeyTask))
			}
		}

		confKeyInput := input.ShortName()
		if tmplOrStaticVal == nil {
			tmplOrStaticVal = p.GetTmplOrTypedValueForConfigKey(confKeyInput, input.TypeName(), currentTask.TaskDef.BindParamsFromEnv)
			if tmplOrStaticVal == nil {
				errs = multierror.Append(errs, fmt.Errorf("no value for config `%s`", confKeyInput))
			}
		}

		inTaskName := p.TaskNamer.FromResolvedInput(input)
		if tmplOrStaticVal == nil {
			inputName := inTaskName.ShortString()
			tmplOrStaticVal = p.GetTmplOrTypedValueForConfigKey(inputName, input.TypeName(), currentTask.TaskDef.BindParamsFromEnv)
			if tmplOrStaticVal == nil {
				errs = multierror.Append(errs, fmt.Errorf("no value for config `%s`", inputName))
			}
		}

		// Missed all the value sources(default, args, params, options)
		pathComponents := strings.Split(input.Name, ".")
		if tmplOrStaticVal == nil {
			var err error
			tmplOrStaticVal, err = maputil.GetValueAtPath(p.CachedTaskOutputs, pathComponents)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			if tmplOrStaticVal == nil {
				args := arguments.GetSubOrEmpty(input.Name)
				var output string
				output, err = p.RunTask(inTaskName, []string{}, args, map[string]interface{}{}, true, currentTask)
				if output != "" {
					tmplOrStaticVal = output
				}
				if err != nil {
					ctx.Debugf("task %#v failed. output was %#v(%T)", inTaskName, tmplOrStaticVal, tmplOrStaticVal)
					ctx.Debug("looking for a default value")
					// Check if any default value is given
					if tmplOrStaticVal == nil {
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
		p.Log.Debugf("tmplOrStaticVal=%#v", tmplOrStaticVal)
		if tmplOrStaticVal != nil {
			var renderedValue string
			expr, ok := tmplOrStaticVal.(string)
			if ok {
				taskTemplate := NewTaskTemplate(currentTask, scope)
				p.Log.Debugf("rendering %s", expr)
				r, err := taskTemplate.Render(expr, input.Name)
				if err != nil {
					return nil, errors.Wrap(err, "failed to render task template")
				}
				renderedValue = r
				p.Log.Debugf("converting type of %v(%T) to %s", renderedValue, renderedValue, input.TypeName())
				tmplOrStaticVal, err = p.parseSupportedValueFromString(renderedValue, input.TypeName())
				if err != nil {
					return nil, err
				}
				p.Log.Debugf("value after type conversion=%v(%T)", tmplOrStaticVal, tmplOrStaticVal)
			}
		} else {
			// the dependent task succeeded with no output
		}

		maputil.SetValueAtPath(values, pathComponents, tmplOrStaticVal)
	}

	ctx.WithField("values", values).Debugf("app finished collecting inputs")

	return values, nil
}

func (p *Application) parseSupportedValueFromString(renderedValue string, typeName string) (interface{}, error) {
	switch typeName {
	case "string":
		p.Log.Debugf("string=%v", renderedValue)
		return renderedValue, nil
	case "integer":
		p.Log.Debugf("integer=%v", renderedValue)
		value, err := strconv.Atoi(renderedValue)
		if err != nil {
			return nil, errors.Wrapf(err, "%v can't be casted to integer", renderedValue)
		}
		return value, nil
	case "boolean":
		p.Log.Debugf("boolean=%v", renderedValue)
		switch renderedValue {
		case "true":
			return true, nil
		case "false":
			return false, nil
		default:
			return nil, fmt.Errorf("%v can't be parsed as boolean", renderedValue)
		}
	case "array", "object":
		p.Log.Debugf("converting this to either array or object=%v", renderedValue)
		var dst interface{}
		if err := yaml.Unmarshal([]byte(renderedValue), &dst); err != nil {
			return nil, errors.Wrapf(err, "failed converting: failed to parse %s as json", renderedValue)
		}
		switch dst.(type) {
		case map[interface{}]interface{}:
			d, err := maputil.RecursivelyStringifyKeys(dst)
			if err != nil {
				return nil, err
			}
			dst = d
		}
		return dst, nil
	default:
		p.Log.Debugf("foobar")
		return nil, fmt.Errorf("unsupported input type `%s` found. the type should be one of: string, integer, boolean", typeName)
	}
}

func (p *Application) Tasks() map[string]*Task {
	return p.TaskRegistry.Tasks()
}

func (p *Application) jsonschemaFromInputs(inputs []*InputConfig) (*gojsonschema.Schema, error) {
	newObjSchema := func() map[string]interface{} {
		return map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
			"required":   []string{},
		}
	}
	root := newObjSchema()
	//map[string]interface{}{}
	for _, input := range inputs {
		name := strings.Replace(input.Name, "-", "_", -1)
		keys := strings.Split(name, ".")
		lastKeyIndex := len(keys) - 1
		for i := range keys {
			var parentSchema map[string]interface{}
			var prop interface{}

			schemaPath := keys[0:i]
			if len(schemaPath) == 0 {
				parentSchema = root
			} else {
				parentSchemaKeys := strings.Split(strings.Join(schemaPath, ".properties."), ".")
				par, err := maputil.GetValueAtPath(root, parentSchemaKeys)
				if err != nil {
					return nil, err
				}
				if par == nil {
					parentSchema = newObjSchema()
					if err := maputil.SetValueAtPath(root, parentSchemaKeys, parentSchema); err != nil {
						return nil, err
					}
				} else {
					parentSchema = par.(map[string]interface{})
				}
			}

			props := parentSchema["properties"].(map[string]interface{})
			required := parentSchema["required"].([]string)

			if i == lastKeyIndex {
				prop = input.JSONSchema()
			} else {
				prop = newObjSchema()
			}
			currentKey := keys[i]
			props[currentKey] = prop

			if input.Required() {
				required = append(required, currentKey)
			}
		}
	}
	p.Log.Debugf("schema = %+v", root)
	schemaLoader := gojsonschema.NewGoLoader(root)
	return gojsonschema.NewSchema(schemaLoader)
}
