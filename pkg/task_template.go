package variant

import (
	"bytes"
	"fmt"
	"github.com/mumoshu/variant/pkg/util/maputil"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"strings"
	"text/template"
)

type TaskTemplate struct {
	task   *Task
	values map[string]interface{}
}

func NewTaskTemplate(task *Task, values map[string]interface{}) *TaskTemplate {
	return &TaskTemplate{
		task:   task,
		values: values,
	}
}

func (f *TaskTemplate) createFuncMap() template.FuncMap {
	get := func(key string) (interface{}, error) {

		sep := "."
		components := strings.Split(strings.Replace(key, "-", "_", -1), sep)
		val, err := maputil.GetValueAtPath(f.values, components)

		if err != nil {
			return nil, errors.WithStack(err)
		}

		if val == nil {
			return nil, fmt.Errorf("no value found for \"%s\"", key)
		}

		return val, nil
	}

	escapeDoubleQuotes := func(str string) (interface{}, error) {
		val := strings.Replace(str, "\"", "\\\"", -1)
		return val, nil
	}

	ctx := templateContext{
		get: get,
	}

	fns := template.FuncMap{
		"get":                get,
		"join":               join,
		"dig":                dig,
		"merge":              merge,
		"readFile":           readFile,
		"toJson":             toJson,
		"toYaml":             toYaml,
		"fromYaml":           fromYaml,
		"toFlags":            ctx.toFlags,
		"validate":           ctx.validate,
		"escapeDoubleQuotes": escapeDoubleQuotes,
	}

	return fns
}

func (t *TaskTemplate) Render(expr string, name string) (string, error) {
	task := t.task
	tmpl := template.New(fmt.Sprintf("%s.definition.yaml: %s.%s.script", task.ProjectName, name, task.Name.ShortString()))
	tmpl.Option("missingkey=error")

	tmpl, err := tmpl.Funcs(t.createFuncMap()).Parse(expr)
	if err != nil {
		log.Errorf("Error: %v", err)
	}

	var buff bytes.Buffer
	if err := tmpl.Execute(&buff, t.values); err != nil {
		return "", errors.Wrapf(err, "failed rendering %s.%s.%s", task.ProjectName, task.Name.ShortString(), name)
	}

	return buff.String(), nil
}
