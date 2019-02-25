package variant

import (
	"fmt"
	"io/ioutil"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

func NewDefaultTaskConfig() *TaskDef {
	return &TaskDef{
		Inputs:   []*InputConfig{},
		TaskDefs: []*TaskDef{},
		Autoenv:  false,
		Steps:    []Step{},
	}
}

func ReadTaskDefFromString(data string) (*TaskDef, error) {
	err, t := ReadTaskDefFromBytes([]byte(data))
	return err, t
}

func ReadTaskDefFromBytes(data []byte) (*TaskDef, error) {
	log.Debugf("%s", string(data))

	c := NewDefaultTaskConfig()
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, errors.Wrapf(err, "yaml.Unmarshal failed: %v", err)
	}
	return c, nil
}

func ReadTaskDefFromFile(path string) (*TaskDef, error) {
	log.Debugf("Loading %s", path)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s does not exist", path)
	}

	yamlBytes, err := ioutil.ReadFile(path)

	if err != nil {
		return nil, fmt.Errorf("Error while loading %s", path)
	}

	t, err := ReadTaskDefFromBytes(yamlBytes)

	if err != nil {
		return nil, errors.Wrapf(err, "Error while loading %s", path)
	}

	return t, nil
}
