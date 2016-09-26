package engine

import (
	"fmt"
	"io/ioutil"
	"os"

	log "github.com/Sirupsen/logrus"

	"github.com/juju/errors"
	"gopkg.in/yaml.v2"
)

func NewDefaultFlowConfig() *FlowConfig {
	return &FlowConfig{
		Inputs:      []*Input{},
		FlowConfigs: []*FlowConfig{},
		Autoenv:     false,
		Steps:       []Step{},
	}
}

func ReadFlowConfigFromString(data string) (*FlowConfig, error) {
	err, t := ReadFlowConfigFromBytes([]byte(data))
	return err, t
}

func ReadFlowConfigFromBytes(data []byte) (*FlowConfig, error) {
	c := NewDefaultFlowConfig()
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, errors.Annotatef(err, "yaml.Unmarshal failed: %v", err)
	}
	return c, nil
}

func ReadFlowConfigFromFile(path string) (*FlowConfig, error) {
	log.Debugf("Loading %s", path)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s does not exist", path)
	}

	yamlBytes, err := ioutil.ReadFile(path)

	if err != nil {
		return nil, fmt.Errorf("Error while loading %s", path)
	}

	log.Debugf("%s", string(yamlBytes))

	t, err := ReadFlowConfigFromBytes(yamlBytes)

	if err != nil {
		return nil, errors.Annotatef(err, "Error while loading %s", path)
	}

	return t, nil
}
