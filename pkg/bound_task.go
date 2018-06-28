package variant

import (
	"strings"
	"text/template"

	log "github.com/Sirupsen/logrus"

	"fmt"
	"github.com/juju/errors"

	"github.com/mumoshu/variant/pkg/util/maputil"

	"github.com/mumoshu/variant/pkg/api/step"
)

type BoundTask struct {
	Task
	Vars map[string]interface{}
}

func (t BoundTask) GetKey() step.Key {
	return t.Name
}

func (t BoundTask) GenerateAutoenv() (map[string]string, error) {
	replacer := strings.NewReplacer("-", "_", ".", "_")
	toEnvName := func(parName string) string {
		return strings.ToUpper(replacer.Replace(parName))
	}
	return t.GenerateAutoenvRecursively("", t.Vars, toEnvName)
}

func (t BoundTask) GenerateAutoenvRecursively(path string, env map[string]interface{}, toEnvName func(string) string) (map[string]string, error) {
	logger := log.WithFields(log.Fields{"path": path})
	result := map[string]string{}
	for k, v := range env {
		if nestedEnv, ok := v.(map[string]interface{}); ok {
			nestedResult, err := t.GenerateAutoenvRecursively(fmt.Sprintf("%s.", k), nestedEnv, toEnvName)
			if err != nil {
				logger.Errorf("Error while recursiong: %v", err)
			}
			for k, v := range nestedResult {
				result[k] = v
			}
		} else if nestedEnv, ok := v.(map[string]string); ok {
			for k2, v := range nestedEnv {
				result[toEnvName(fmt.Sprintf("%s%s.%s", path, k, k2))] = v
			}
		} else if ary, ok := v.([]string); ok {
			for i, v := range ary {
				result[toEnvName(fmt.Sprintf("%s%s.%d", path, k, i))] = v
			}
		} else {
			if stringV, ok := v.(string); ok {
				result[toEnvName(fmt.Sprintf("%s%s", path, k))] = stringV
			} else if v == nil {
				result[toEnvName(fmt.Sprintf("%s%s", path, k))] = ""
			} else {
				return nil, errors.Errorf("The value for the key %s was neither a `map[string]interface{}` nor a `string`: %v(%#v)", k, v, v)
			}
		}
	}
	logger.Debugf("Generated autoenv: %v", result)
	return result, nil
}

func (t *BoundTask) Run(project *Application, caller ...step.Caller) (string, error) {
	var ctx *log.Entry

	if len(caller) > 0 {
		ctx = log.WithFields(log.Fields{"caller": caller[0].GetKey().ShortString()})
	} else {
		ctx = log.WithFields(log.Fields{})
	}

	ctx.Debugf("task %s started", t.Name.String())

	var output step.StepStringOutput
	var err error

	context := NewExecutionContextImpl(*project, *t)

	for _, step := range t.Steps {
		output, err = step.Run(context)

		if err != nil {
			return "", errors.Annotate(err, "Task#Run failed while running a script")
		}
	}

	if err != nil {
		err = errors.Annotate(err, "Task#Run failed while running a script")
	}

	ctx.Debugf("task %s finished", t.Name.String())

	return output.String, err
}

func (f BoundTask) CreateFuncMap() template.FuncMap {
	get := func(key string) (interface{}, error) {

		sep := "."
		components := strings.Split(strings.Replace(key, "-", "_", -1), sep)
		val, err := maputil.GetValueAtPath(f.Vars, components)

		if err != nil {
			return nil, errors.Trace(err)
		}
		return val, nil
	}

	escapeDoubleQuotes := func(str string) (interface{}, error) {
		val := strings.Replace(str, "\"", "\\\"", -1)
		return val, nil
	}

	fns := template.FuncMap{
		"get":                get,
		"escapeDoubleQuotes": escapeDoubleQuotes,
	}

	return fns
}
