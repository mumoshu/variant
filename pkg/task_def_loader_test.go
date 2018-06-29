package variant

import (
	log "github.com/Sirupsen/logrus"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
)

const minimalConfigYaml = `
tasks:
  foo:
    bar:
      script: foobar
`

func TestMinimalConfigParsing(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	expected := &TaskDef{
		Inputs: []*InputConfig{},
		TaskDefs: []*TaskDef{
			&TaskDef{
				Autoenv: true,
				Autodir: true,
				Name:    "foo",
				Inputs:  []*InputConfig{},
				TaskDefs: []*TaskDef{
					&TaskDef{
						Autoenv:  true,
						Autodir:  true,
						Name:     "bar",
						Script:   "foobar",
						TaskDefs: []*TaskDef{},
						Inputs:   []*InputConfig{},
					},
				},
			},
		},
		Autoenv: true,
		Autodir: true,
	}
	actual, err := ReadTaskConfigFromString(minimalConfigYaml)

	if err != nil {
		t.Errorf("Error: %v", err)
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("actual value %s doesn't match expected value %s in config %s", spew.Sdump(actual), spew.Sdump(expected), minimalConfigYaml)
	}
}
