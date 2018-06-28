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
	expected := &TaskConfig{
		Inputs: []*Input{},
		TaskConfigs: []*TaskConfig{
			&TaskConfig{
				Autoenv: true,
				Autodir: true,
				Name:    "foo",
				Inputs:  []*Input{},
				TaskConfigs: []*TaskConfig{
					&TaskConfig{
						Autoenv:     true,
						Autodir:     true,
						Name:        "bar",
						Script:      "foobar",
						TaskConfigs: []*TaskConfig{},
						Inputs:      []*Input{},
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
