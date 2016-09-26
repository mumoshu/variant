package engine

import (
	"strings"
	"text/template"

	log "github.com/Sirupsen/logrus"

	"fmt"
	"github.com/juju/errors"

	"../util/maputil"
)

type BoundFlow struct {
	Flow
	Vars map[string]interface{}
}

func (t BoundFlow) GenerateAutoenv() (map[string]string, error) {
	replacer := strings.NewReplacer("-", "_", ".", "_")
	toEnvName := func(parName string) string {
		return strings.ToUpper(replacer.Replace(parName))
	}
	return t.GenerateAutoenvRecursively("", t.Vars, toEnvName)
}

func (t BoundFlow) GenerateAutoenvRecursively(path string, env map[string]interface{}, toEnvName func(string) string) (map[string]string, error) {
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
			} else {
				return nil, errors.Errorf("The value for the key %s was neither a `map[string]interface{}` nor a `string`: %v", k, v)
			}
		}
	}
	logger.Debugf("Generated autoenv: %v", result)
	return result, nil
}

func (t *BoundFlow) Run(project *Application, caller ...Flow) (string, error) {
	if len(caller) > 0 {
		log.Debugf("running flow `%s` via `%s`", t.Key.String(), caller[0].Key.String())
	} else {
		log.Infof("running flow: %s", t.Key.String())
	}

	var output StepStringOutput
	var err error

	for _, step := range t.Steps {
		output, err = step.Run(project, t, caller...)

		if err != nil {
			return "", errors.Annotate(err, "Flow#Run failed while running a script")
		}
	}

	if err != nil {
		err = errors.Annotate(err, "Flow#Run failed while running a script")
	}

	return output.String, err
}

func (f BoundFlow) CreateFuncMap() template.FuncMap {
	get := func(key string) (interface{}, error) {
		val, err := maputil.GetStringAtPath(f.Vars, key)

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
