package variant

import (
	"fmt"
	"reflect"

	log "github.com/Sirupsen/logrus"
	"github.com/juju/errors"
	"github.com/mitchellh/mapstructure"

	"github.com/mumoshu/variant/pkg/api/step"
	"github.com/mumoshu/variant/pkg/util/maputil"
)

type TaskDef struct {
	Name        string         `yaml:"name,omitempty"`
	Description string         `yaml:"description,omitempty"`
	Inputs      []*InputConfig `yaml:"inputs,omitempty"`
	TaskDefs    []*TaskDef     `yaml:"tasks,omitempty"`
	Script      string         `yaml:"script,omitempty"`
	Steps       []step.Step    `yaml:"steps,omitempty"`
	Autoenv     bool           `yaml:"autoenv,omitempty"`
	Autodir     bool           `yaml:"autodir,omitempty"`
	Interactive bool           `yaml:"interactive,omitempty"`
}

type TaskDefV1 struct {
	Name        string                        `yaml:"name,omitempty"`
	Description string                        `yaml:"description,omitempty"`
	Inputs      []*InputConfig                `yaml:"inputs,omitempty"`
	Parameters  []*ParameterConfig            `yaml:"parameters,omitempty"`
	Options     []*OptionConfig               `yaml:"options,omitempty"`
	TaskDefs    []*TaskDef                    `yaml:"tasks,omitempty"`
	Runner      map[string]interface{}        `yaml:"runner,omitempty"`
	Script      string                        `yaml:"script,omitempty"`
	StepDefs    []map[interface{}]interface{} `yaml:"steps,omitempty"`
	Autoenv     bool                          `yaml:"autoenv,omitempty"`
	Autodir     bool                          `yaml:"autodir,omitempty"`
	Interactive bool                          `yaml:"interactive,omitempty"`
}

type TaskDefV2 struct {
	Description string                        `yaml:"description,omitempty"`
	Inputs      []*InputConfig                `yaml:"inputs,omitempty"`
	Parameters  []*ParameterConfig            `yaml:"parameters,omitempty"`
	Options     []*OptionConfig               `yaml:"options,omitempty"`
	TaskDefs    map[string]*TaskDef           `yaml:"tasks,omitempty"`
	Runner      map[string]interface{}        `yaml:"runner,omitempty"`
	Script      string                        `yaml:"script,omitempty"`
	StepDefs    []map[interface{}]interface{} `yaml:"steps,omitempty"`
	Autoenv     bool                          `yaml:"autoenv,omitempty"`
	Autodir     bool                          `yaml:"autodir,omitempty"`
	Interactive bool                          `yaml:"interactive,omitempty"`
}

func (t *TaskDef) UnmarshalYAML(unmarshal func(interface{}) error) error {
	v3 := map[string]interface{}{}
	v3err := unmarshal(&v3)

	log.Debugf("Unmarshalling: %v", v3)

	log.Debugf("Trying to parse v1 format")

	v1 := TaskDefV1{
		Autoenv:  false,
		Autodir:  false,
		Inputs:   []*InputConfig{},
		TaskDefs: []*TaskDef{},
		StepDefs: []map[interface{}]interface{}{},
	}

	err := unmarshal(&v1)

	if v1.Name == "" && len(v1.TaskDefs) == 0 {
		e := fmt.Errorf("Not v1 format: Both `name` and `tasks` are empty")
		log.Debugf("%s", e)
		err = e
	}

	if err == nil {
		t.Name = v1.Name
		t.Description = v1.Description
		t.Inputs = v1.Inputs
		if len(v1.Inputs) > 0 {
			t.Inputs = v1.Inputs
		} else {
			for i, p := range v1.Parameters {
				c := i
				input := &InputConfig{
					Name:          p.Name,
					Description:   p.Description,
					ArgumentIndex: &c,
					Type:          p.Type,
					Default:       p.Default,
					Remainings:    p.Remainings,
				}
				t.Inputs = append(t.Inputs, input)
			}
			for _, o := range v1.Options {
				input := &InputConfig{
					Name:        o.Name,
					Description: o.Description,
					Type:        o.Type,
					Default:     o.Default,
					Remainings:  o.Remainings,
				}
				t.Inputs = append(t.Inputs, input)
			}
		}
		t.TaskDefs = v1.TaskDefs
		t.Script = v1.Script
		t.Autoenv = v1.Autoenv
		t.Autodir = v1.Autodir
		t.Interactive = v1.Interactive
		steps, err := readStepsFromStepDefs(v1.Script, v1.Runner, v1.StepDefs)
		if err != nil {
			return errors.Annotatef(err, "Error while reading v1 config")
		}
		t.Steps = steps
	}

	var v2 *TaskDefV2

	if err != nil {
		log.Debugf("Trying to parse v2 format")
		v2 = &TaskDefV2{
			Autoenv:     false,
			Autodir:     false,
			Interactive: false,
			Inputs:      []*InputConfig{},
			TaskDefs:    map[string]*TaskDef{},
			StepDefs:    []map[interface{}]interface{}{},
		}

		err = unmarshal(&v2)

		if len(v2.TaskDefs) == 0 && v2.Script == "" && len(v2.StepDefs) == 0 {
			e := fmt.Errorf("Not v2 format: `tasks`, `script`, `steps` are missing.")
			log.Debugf("%s", e)
			err = e
		}

		if err == nil {
			t.Description = v2.Description
			if len(v2.Inputs) > 0 {
				t.Inputs = v2.Inputs
			} else {
				for i, p := range v2.Parameters {
					c := i
					input := &InputConfig{
						Name:          p.Name,
						Description:   p.Description,
						ArgumentIndex: &c,
						Type:          p.Type,
						Default:       p.Default,
						Remainings:    p.Remainings,
					}
					t.Inputs = append(t.Inputs, input)
				}
				for _, o := range v2.Options {
					input := &InputConfig{
						Name:        o.Name,
						Description: o.Description,
						Type:        o.Type,
						Default:     o.Default,
						Remainings:  o.Remainings,
					}
					t.Inputs = append(t.Inputs, input)
				}
			}
			t.TaskDefs = TransformV2FlowConfigMapToArray(v2.TaskDefs)
			steps, err := readStepsFromStepDefs(v2.Script, v2.Runner, v2.StepDefs)
			if err != nil {
				return errors.Annotatef(err, "Error while reading v2 config")
			}
			t.Steps = steps
			t.Script = v2.Script
			t.Autoenv = v2.Autoenv
			t.Autodir = v2.Autodir
			t.Interactive = v2.Interactive
		}

	}

	if err != nil {
		log.Debugf("Trying to parse v3 format")

		if v3err != nil {
			return errors.Annotate(v3err, "Failed to unmarshal as a map.")
		}

		if v3["tasks"] != nil {
			rawTasks, ok := v3["tasks"].(map[interface{}]interface{})
			if !ok {
				return fmt.Errorf("Not a map[interface{}]interface{}: v3[\"tasks\"]'s type: %s, value: %s", reflect.TypeOf(v3["tasks"]), v3["tasks"])
			}
			tasks, err := maputil.CastKeysToStrings(rawTasks)
			if err != nil {
				return errors.Annotate(err, "Failed to unmarshal as a map[string]interface{}")
			}
			t.Autoenv = false
			t.Autodir = false
			t.Inputs = []*InputConfig{}

			t.TaskDefs = TransformV3TaskDefMapToArray(tasks)

			return nil
		}
	}

	return errors.Trace(err)
}

func (t *TaskDef) CopyTo(other *TaskDef) {
	other.Description = t.Description
	other.Inputs = t.Inputs
	other.TaskDefs = t.TaskDefs
	other.Steps = t.Steps
	other.Script = t.Script
	other.Autoenv = t.Autoenv
	other.Autodir = t.Autodir
	other.Interactive = t.Interactive
}

func TransformV2FlowConfigMapToArray(v2 map[string]*TaskDef) []*TaskDef {
	result := []*TaskDef{}
	for name, t2 := range v2 {
		t := &TaskDef{}

		t.Name = name
		t2.CopyTo(t)

		result = append(result, t)
	}
	return result
}

var stepLoaders []step.StepLoader

func Register(stepLoader step.StepLoader) {
	stepLoaders = append(stepLoaders, stepLoader)
}

func init() {
	stepLoaders = []step.StepLoader{}
}

type stepLoadingContextImpl struct{}

func (s stepLoadingContextImpl) LoadStep(config step.StepDef) (step.Step, error) {
	step, err := LoadStep(config)

	return step, err
}

func LoadStep(config step.StepDef) (step.Step, error) {
	var lastError error

	lastError = nil

	context := stepLoadingContextImpl{}
	for _, loader := range stepLoaders {
		var s step.Step
		s, lastError = loader.LoadStep(config, context)

		log.WithField("step", s).Debugf("step loaded")

		if lastError == nil {
			return s, nil
		}
	}
	return nil, errors.Annotatef(lastError, "all loader failed to load step")
}

func readStepsFromStepDefs(script string, runner map[string]interface{}, stepDefs []map[interface{}]interface{}) ([]step.Step, error) {
	result := []step.Step{}

	if script != "" {
		if len(stepDefs) > 0 {
			return nil, fmt.Errorf("both script and steps exist.")
		}

		raw := map[string]interface{}{
			"name":   "script",
			"script": script,
			"silent": false,
		}
		if runner != nil {
			raw["runner"] = runner
		}
		s, err := LoadStep(step.NewStepDef(raw))

		if err != nil {
			log.Panicf("step failed to load: %v", err)
		}

		result = []step.Step{s}
	} else {
		for i, stepDef := range stepDefs {
			defaultName := fmt.Sprintf("step-%d", i+1)

			if stepDef["name"] == "" || stepDef["name"] == nil {
				stepDef["name"] = defaultName
			}

			converted, castErr := maputil.CastKeysToStrings(stepDef)

			if castErr != nil {
				panic(castErr)
			}

			s, err := LoadStep(step.NewStepDef(converted))

			if err != nil {
				return nil, errors.Annotatef(err, "Error reading step[%d]")
			}

			result = append(result, s)
		}
	}

	return result, nil
}

func TransformV3TaskDefMapToArray(v3 map[string]interface{}) []*TaskDef {
	result := []*TaskDef{}
	for k, v := range v3 {
		t := &TaskDef{
			Autoenv:  false,
			Autodir:  false,
			Inputs:   []*InputConfig{},
			TaskDefs: []*TaskDef{},
		}

		log.Debugf("Arrived %s: %v", k, v)
		log.Debugf("Type of value: %v", reflect.TypeOf(v))

		t.Name = k

		var err error

		i2i, ok := v.(map[interface{}]interface{})

		if !ok {
			panic(fmt.Errorf("Not a map[interface{}]interface{}: %s", v))
		}

		s2i, err := maputil.CastKeysToStrings(i2i)

		if err != nil {
			panic(errors.Annotate(err, "Unexpected structure"))
		}

		leaf := s2i["script"] != nil

		if !leaf {
			t.TaskDefs = TransformV3TaskDefMapToArray(s2i)
		} else {
			log.Debugf("Not a nested map")
			err = mapstructure.Decode(s2i, t)
			if err != nil {
				panic(errors.Trace(err))
			}
			log.Debugf("Loaded %v", t)
		}

		result = append(result, t)
	}
	return result
}
