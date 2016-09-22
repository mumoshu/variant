package engine

import (
	"strings"
	"text/template"

	log "github.com/Sirupsen/logrus"

	"github.com/juju/errors"
)

func (t *Flow) Run(project *Project, caller ...FlowDef) (string, error) {
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

func (f Flow) CreateFuncMap() template.FuncMap {
	get := func(key string) (interface{}, error) {
		val, err := recursiveFetchFromMap(f.Vars, key)

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
