package engine

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"

	"github.com/juju/errors"
	"gopkg.in/yaml.v2"
)

func ParseEnviron() map[string]string {
	mergedEnv := map[string]string{}

	for _, pair := range os.Environ() {
		splits := strings.SplitN(pair, "=", 2)
		key, value := splits[0], splits[1]
		mergedEnv[key] = value
	}

	return mergedEnv
}

func NewDefaultFlowConfig() *FlowConfig {
	return &FlowConfig{
		Inputs:      []*Input{},
		FlowConfigs: []*FlowConfig{},
		Autoenv:     false,
		Steps:       []Step{},
	}
}

func FetchCache(cache map[string]interface{}, keyComponents []string) (interface{}, error) {
	k, rest := keyComponents[0], keyComponents[1:]

	k = strings.Replace(k, "-", "_", -1)

	if len(rest) == 0 {
		return cache[k], nil
	} else {
		nested, ok := cache[k].(map[string]interface{})
		if ok {
			v, err := FetchCache(nested, rest)
			if err != nil {
				return nil, err
			}
			return v, nil
		} else if cache[k] != nil {
			return nil, errors.Errorf("%s is not a map[string]interface{}", k)
		} else {
			return nil, nil
		}
	}
}

func SetValueAtPath(cache map[string]interface{}, keyComponents []string, value interface{}) error {
	k, rest := keyComponents[0], keyComponents[1:]

	k = strings.Replace(k, "-", "_", -1)

	if len(rest) == 0 {
		cache[k] = value
	} else {
		_, ok := cache[k].(map[string]interface{})
		if !ok && cache[k] != nil {
			return errors.Errorf("%s is not an map[string]interface{}", k)
		}
		if cache[k] == nil {
			cache[k] = map[string]interface{}{}
		}
		err := SetValueAtPath(cache[k].(map[string]interface{}), rest, value)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func recursiveFetchFromMap(m map[string]interface{}, key string) (string, error) {
	sep := "."

	components := strings.Split(strings.Replace(key, "-", "_", -1), sep)
	log.Debugf("components=%v", components)
	head := components[0]
	rest := components[1:]
	value, exists := m[head]
	if !exists {
		return "", fmt.Errorf("No value for %s in %+v", head, m)
	}

	next, isMap := value.(map[string]interface{})
	result, isStr := value.(string)

	if !isStr {
		if !isMap {
			return "", fmt.Errorf("Not map or string: %s in %+v", head, m)
		}

		if len(rest) == 0 {
			return "", fmt.Errorf("%s in %+v is a map but no more key to recurse", head, m)
		}

		return recursiveFetchFromMap(next, strings.Join(rest, sep))
	}

	return result, nil
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
