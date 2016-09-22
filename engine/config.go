package engine

import (
	"fmt"
	"reflect"

	log "github.com/Sirupsen/logrus"
	"github.com/juju/errors"
	"github.com/mitchellh/mapstructure"
)

type FlowConfig struct {
	Name        string        `yaml:"name,omitempty"`
	Description string        `yaml:"description,omitempty"`
	Inputs      []*Input      `yaml:"inputs,omitempty"`
	FlowConfigs []*FlowConfig `yaml:"flows,omitempty"`
	Script      string        `yaml:"script,omitempty"`
	Steps       []Step        `yaml:"steps,omitempty"`
	Autoenv     bool          `yaml:"autoenv,omitempty"`
	Autodir     bool          `yaml:"autodir,omitempty"`
	Interactive bool          `yaml:"interactive,omitempty"`
}

type FlowConfigV1 struct {
	Name        string        `yaml:"name,omitempty"`
	Description string        `yaml:"description,omitempty"`
	Inputs      []*Input      `yaml:"inputs,omitempty"`
	FlowConfigs []*FlowConfig `yaml:"flows,omitempty"`
	Script      string        `yaml:"script,omitempty"`
	StepConfigs []*StepConfig `yaml:"steps,omitempty"`
	Autoenv     bool          `yaml:"autoenv,omitempty"`
	Autodir     bool          `yaml:"autodir,omitempty"`
	Interactive bool          `yaml:"interactive,omitempty"`
}

type FlowConfigV2 struct {
	Description string                 `yaml:"description,omitempty"`
	Inputs      []*Input               `yaml:"inputs,omitempty"`
	FlowConfigs map[string]*FlowConfig `yaml:"flows,omitempty"`
	Script      string                 `yaml:"script,omitempty"`
	StepConfigs []*StepConfig          `yaml:"steps,omitempty"`
	Autoenv     bool                   `yaml:"autoenv,omitempty"`
	Autodir     bool                   `yaml:"autodir,omitempty"`
	Interactive bool                   `yaml:"interactive,omitempty"`
}

type StepConfig struct {
	Name   string      `yaml:"name,omitempty"`
	Script interface{} `yaml:"script,omitempty"`
	Flow   interface{} `yaml:"flow,omitempty"`
}

func (t *FlowConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	v3 := map[string]interface{}{}
	v3err := unmarshal(&v3)

	log.Debugf("Unmarshalling: %v", v3)

	log.Debugf("Trying to parse v1 format")

	v1 := FlowConfigV1{
		Autoenv:     false,
		Autodir:     false,
		Inputs:      []*Input{},
		FlowConfigs: []*FlowConfig{},
		StepConfigs: []*StepConfig{},
	}

	err := unmarshal(&v1)

	if v1.Name == "" && len(v1.FlowConfigs) == 0 {
		e := fmt.Errorf("Not v1 format: Both `name` and `flows` are empty")
		log.Debugf("%s", e)
		err = e
	}

	if err == nil {
		t.Name = v1.Name
		t.Description = v1.Description
		t.Inputs = v1.Inputs
		t.FlowConfigs = v1.FlowConfigs
		t.Script = v1.Script
		t.Autoenv = v1.Autoenv
		t.Autodir = v1.Autodir
		t.Interactive = v1.Interactive
		steps, err := readStepsFromStepConfigs(v1.Script, v1.StepConfigs)
		if err != nil {
			return errors.Annotatef(err, "Error while reading v1 config")
		}
		t.Steps = steps
	}

	var v2 *FlowConfigV2

	if err != nil {
		log.Debugf("Trying to parse v2 format")
		v2 = &FlowConfigV2{
			Autoenv:     false,
			Autodir:     false,
			Interactive: false,
			Inputs:      []*Input{},
			FlowConfigs: map[string]*FlowConfig{},
			StepConfigs: []*StepConfig{},
		}

		err = unmarshal(&v2)

		if len(v2.FlowConfigs) == 0 && v2.Script == "" && len(v2.StepConfigs) == 0 {
			e := fmt.Errorf("Not v2 format: `flows`, `script`, `steps` are missing.")
			log.Debugf("%s", e)
			err = e
		}

		if err == nil {
			t.Description = v2.Description
			t.Inputs = v2.Inputs
			t.FlowConfigs = TransformV2FlowConfigMapToArray(v2.FlowConfigs)
			steps, err := readStepsFromStepConfigs(v2.Script, v2.StepConfigs)
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

		if v3["flows"] != nil {
			rawFlows, ok := v3["flows"].(map[interface{}]interface{})
			if !ok {
				return fmt.Errorf("Not a map[interface{}]interface{}: v3[\"flows\"]'s type: %s, value: %s", reflect.TypeOf(v3["flows"]), v3["flows"])
			}
			flows, err := CastKeysToStrings(rawFlows)
			if err != nil {
				return errors.Annotate(err, "Failed to unmarshal as a map[string]interface{}")
			}
			t.Autoenv = false
			t.Autodir = false
			t.Inputs = []*Input{}

			t.FlowConfigs = TransformV3FlowConfigMapToArray(flows)

			return nil
		}
	}

	return errors.Trace(err)
}

func (t *FlowConfig) CopyTo(other *FlowConfig) {
	other.Description = t.Description
	other.Inputs = t.Inputs
	other.FlowConfigs = t.FlowConfigs
	other.Steps = t.Steps
	other.Script = t.Script
	other.Autoenv = t.Autoenv
	other.Autodir = t.Autodir
	other.Interactive = t.Interactive
}

func TransformV2FlowConfigMapToArray(v2 map[string]*FlowConfig) []*FlowConfig {
	result := []*FlowConfig{}
	for name, t2 := range v2 {
		t := &FlowConfig{}

		t.Name = name
		t2.CopyTo(t)

		result = append(result, t)
	}
	return result
}

type StepLoader interface {
	TryToLoad(stepConfig StepConfig) Step
}

var stepLoaders []StepLoader

func Register(stepLoader StepLoader) {
	stepLoaders = append(stepLoaders, stepLoader)
}

func init() {
	stepLoaders = []StepLoader{}
}

func TryToLoad(stepConfig StepConfig) Step {
	for _, loader := range stepLoaders {
		s := loader.TryToLoad(stepConfig)
		if s != nil {
			return s
		}
	}
	return nil
}

func readStepsFromStepConfigs(script string, stepConfigs []*StepConfig) ([]Step, error) {
	result := []Step{}

	if script != "" {
		if len(stepConfigs) > 0 {
			return nil, fmt.Errorf("both script and steps exist.")
		}

		step := TryToLoad(StepConfig{
			Name:   "script",
			Script: script,
		})

		if step == nil {
			panic("No loader defined for script")
		}

		result = []Step{step}
	} else {

		for i, stepConfig := range stepConfigs {
			var step Step

			defaultName := fmt.Sprintf("step-%d", i+1)

			if stepConfig.Name == "" {
				stepConfig.Name = defaultName
			}

			step = TryToLoad(*stepConfig)

			if step == nil {
				return nil, fmt.Errorf("Error reading step[%d]: field named `flow` or `script` doesn't exist")
			}

			result = append(result, step)
		}
	}

	return result, nil
}

func CastKeysToStrings(m map[interface{}]interface{}) (map[string]interface{}, error) {
	r := map[string]interface{}{}
	for k, v := range m {
		str, ok := k.(string)
		if !ok {
			return nil, fmt.Errorf("Unexpected type %s for key %s", reflect.TypeOf(k), k)
		}
		r[str] = v
	}
	return r, nil
}

func TransformV3FlowConfigMapToArray(v3 map[string]interface{}) []*FlowConfig {
	result := []*FlowConfig{}
	for k, v := range v3 {
		t := &FlowConfig{
			Autoenv:     false,
			Autodir:     false,
			Inputs:      []*Input{},
			FlowConfigs: []*FlowConfig{},
		}

		log.Debugf("Arrived %s: %v", k, v)
		log.Debugf("Type of value: %v", reflect.TypeOf(v))

		t.Name = k

		var err error

		i2i, ok := v.(map[interface{}]interface{})

		if !ok {
			panic(fmt.Errorf("Not a map[interface{}]interface{}: %s", v))
		}

		s2i, err := CastKeysToStrings(i2i)

		if err != nil {
			panic(errors.Annotate(err, "Unexpected structure"))
		}

		leaf := s2i["script"] != nil

		if !leaf {
			t.FlowConfigs = TransformV3FlowConfigMapToArray(s2i)
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
