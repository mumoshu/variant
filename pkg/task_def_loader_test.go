package variant

import (
	"github.com/google/go-cmp/cmp/cmpopts"
	log "github.com/sirupsen/logrus"
	"testing"

	"github.com/google/go-cmp/cmp"
)

const minimalConfigYaml = `
tasks:
  foo:
    tasks:
      bar:
        script: foobar
`

func TestMinimalConfigParsing(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	expected := &TaskDef{
		Inputs: InputConfigs{},
		TaskDefs: []*TaskDef{
			&TaskDef{
				Autoenv: false,
				Autodir: false,
				Name:    "foo",
				Inputs:  nil,
				Steps: []Step{},
				TaskDefs: []*TaskDef{
					&TaskDef{
						Autoenv:  false,
						Autodir:  false,
						Name:     "bar",
						Script:   "foobar",
						Steps: []Step{nil},
						TaskDefs: []*TaskDef{},
						Inputs:   nil,
					},
				},
			},
		},
		Steps: []Step{},
		Autoenv: false,
		Autodir: false,
	}
	actual, err := ReadTaskDefFromString(minimalConfigYaml)

	if err != nil {
		t.Errorf("Error: %v", err)
	}

	if diff := cmp.Diff(expected, actual, cmpopts.IgnoreUnexported(TaskDef{})); diff != "" {
		t.Errorf("ReadTaskDefFromString() mismatch (-want +got):\n%s", diff)
	}
}
