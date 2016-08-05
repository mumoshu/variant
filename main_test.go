package main

import (
	log "github.com/Sirupsen/logrus"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
)

const minimalConfigYaml = `
flows:
  foo:
    bar:
      script: foobar
`

func TestMinimalConfigParsing(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	expected := &Target{
		Inputs: []*Input{},
		Targets: []*Target{
			&Target{
				Autoenv: true,
				Autodir: true,
				Name:    "foo",
				Inputs:  []*Input{},
				Targets: []*Target{
					&Target{
						Autoenv: true,
						Autodir: true,
						Name:    "bar",
						Script:  "foobar",
						Targets: []*Target{},
						Inputs:  []*Input{},
					},
				},
			},
		},
		Autoenv: true,
		Autodir: true,
	}
	actual, err := ReadFromString(minimalConfigYaml)

	if err != nil {
		t.Errorf("Error: %v", err)
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("actual value %s doesn't match expected value %s in config %s", spew.Sdump(actual), spew.Sdump(expected), minimalConfigYaml)
	}
}
