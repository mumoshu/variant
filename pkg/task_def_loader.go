package variant

import (
	"fmt"
	"io/ioutil"
	"os"

	log "github.com/Sirupsen/logrus"

	"github.com/juju/errors"
	"github.com/mumoshu/variant/pkg/api/step"
	"gopkg.in/yaml.v2"
)

func NewDefaultTaskConfig() *TaskDef {
	return &TaskDef{
		Inputs:   []*InputConfig{},
		TaskDefs: []*TaskDef{},
		Autoenv:  false,
		Steps:    []step.Step{},
	}
}

func ReadTaskConfigFromString(data string) (*TaskDef, error) {
	err, t := ReadTaskConfigFromBytes([]byte(data))
	return err, t
}

func ReadTaskConfigFromBytes(data []byte) (*TaskDef, error) {
	c := NewDefaultTaskConfig()
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, errors.Annotatef(err, "yaml.Unmarshal failed: %v", err)
	}
	return c, nil
}

func ReadTaskConfigFromFile(path string) (*TaskDef, error) {
	log.Debugf("Loading %s", path)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s does not exist", path)
	}

	yamlBytes, err := ioutil.ReadFile(path)

	if err != nil {
		return nil, fmt.Errorf("Error while loading %s", path)
	}

	log.Debugf("%s", string(yamlBytes))

	t, err := ReadTaskConfigFromBytes(yamlBytes)

	if err != nil {
		return nil, errors.Annotatef(err, "Error while loading %s", path)
	}

	return t, nil
}
