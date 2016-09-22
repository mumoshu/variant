package engine

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"

	"github.com/juju/errors"
	"github.com/spf13/cobra"
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

func newDefaultProjectConfig() *ProjectConfig {
	return &ProjectConfig{
		Inputs:      []*Input{},
		FlowConfigs: []*FlowConfig{},
	}
}

func NewDefaultFlowConfig() *FlowConfig {
	return &FlowConfig{
		Inputs:      []*Input{},
		FlowConfigs: []*FlowConfig{},
		Autoenv:     false,
		Steps:       []Step{},
	}
}

type ProjectConfig struct {
	Name        string
	Description string        `yaml:"description,omitempty"`
	Inputs      []*Input      `yaml:"inputs,omitempty"`
	FlowConfigs []*FlowConfig `yaml:"flows,omitempty"`
	Script      string        `yaml:"script,omitempty"`
}

type FlowDef struct {
	Key         FlowKey
	ProjectName string
	Steps       []Step
	Inputs      []*Input
	Variables   []*Variable
	Autoenv     bool
	Autodir     bool
	Interactive bool
	FlowConfig  *FlowConfig
	Command     *cobra.Command
}

type Flow struct {
	Key         FlowKey
	ProjectName string
	Steps       []Step
	Vars        map[string]interface{}
	Autoenv     bool
	Autodir     bool
	Interactive bool
	FlowDef     *FlowDef
}

type FlowKey struct {
	Components []string
}

type T struct {
	A string
	B struct {
		RenamedC int   `yaml:"c"`
		D        []int `yaml:",flow"`
	}
}

func (t Flow) GenerateAutoenv() (map[string]string, error) {
	replacer := strings.NewReplacer("-", "_", ".", "_")
	toEnvName := func(parName string) string {
		return strings.ToUpper(replacer.Replace(parName))
	}
	return t.GenerateAutoenvRecursively("", t.Vars, toEnvName)
}

func (t Flow) GenerateAutoenvRecursively(path string, env map[string]interface{}, toEnvName func(string) string) (map[string]string, error) {
	logger := log.WithFields(log.Fields{"path": path})
	result := map[string]string{}
	for k, v := range env {
		if nestedEnv, ok := v.(map[string]interface{}); ok {
			nestedResult, err := t.GenerateAutoenvRecursively(fmt.Sprintf("%s.", k), nestedEnv, toEnvName)
			if err != nil {
				logger.Errorf("Error while recursiong: %v", err)
			}
			for k, v := range nestedResult {
				result[k] = v
			}
		} else if nestedEnv, ok := v.(map[string]string); ok {
			for k2, v := range nestedEnv {
				result[toEnvName(fmt.Sprintf("%s%s.%s", path, k, k2))] = v
			}
		} else if ary, ok := v.([]string); ok {
			for i, v := range ary {
				result[toEnvName(fmt.Sprintf("%s%s.%d", path, k, i))] = v
			}
		} else {
			if stringV, ok := v.(string); ok {
				result[toEnvName(fmt.Sprintf("%s%s", path, k))] = stringV
			} else {
				return nil, errors.Errorf("The value for the key %s was neither a `map[string]interface{}` nor a `string`: %v", k, v)
			}
		}
	}
	logger.Debugf("Generated autoenv: %v", result)
	return result, nil
}

type MessageOnlyFormatter struct {
}

func (f *MessageOnlyFormatter) Format(entry *log.Entry) ([]byte, error) {
	return append([]byte(entry.Message), '\n'), nil
}

func (t FlowKey) String() string {
	return strings.Join(t.Components, ".")
}

func (t FlowKey) ShortString() string {
	return strings.Join(t.Components[1:], ".")
}

func (t FlowKey) Parent() (*FlowKey, error) {
	if len(t.Components) > 1 {
		return &FlowKey{Components: t.Components[:len(t.Components)-1]}, nil
	} else {
		return nil, errors.Errorf("FlowKey %v doesn't have a parent", t)
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
