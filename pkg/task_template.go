package variant

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/juju/errors"
	"github.com/mumoshu/variant/pkg/util/maputil"
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

		switch val.(type) {
		case map[string]interface{}, []interface{}:
			bytes, err := json.Marshal(val)
			if err != nil {
				log.Panicf("unexpected error: %v", err)
			}
			val = string(bytes)
		default:
			val = val
		}

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
		return "", errors.Annotatef(err, "failed rendering %s.%s.%s", task.ProjectName, task.Name.ShortString(), name)
	}

	return buff.String(), nil
}
