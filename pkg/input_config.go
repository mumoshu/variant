package variant

import (
	"fmt"
	"reflect"
)

func Int(v int) *int {
	return &v
}

type InputConfig struct {
	Name          string                            `yaml:"name,omitempty"`
	Description   string                            `yaml:"description,omitempty"`
	ArgumentIndex *int                              `yaml:"argument-index,omitempty"`
	Type          string                            `yaml:"type,omitempty"`
	Default       interface{}                       `yaml:"default,omitempty"`
	Properties    map[string]map[string]interface{} `yaml:"properties,omitempty"`
	Remainings    map[string]interface{}            `yaml:",inline"`
}

func (c *InputConfig) GoString() string {
	var argIdx string
	if c.ArgumentIndex == nil {
		argIdx = "nil"
	} else {
		argIdx = fmt.Sprintf("variant.Int(%d)", *c.ArgumentIndex)
	}
	var def string
	if c.Default == nil {
		def = "nil"
	} else {
		def = fmt.Sprintf("%#v", c.Default)
	}

	return fmt.Sprintf(
		`&variant.InputConfig{Name:%#v, Description:%#v, ArgumentIndex:%s, Type:%#v, Default:%s, Properties:%#v, Remainings:%#v}`,
		c.Name, c.Description, argIdx, c.Type, def, c.Properties, c.Remainings,
	)
}

func (c *InputConfig) Required() bool {
	return c.Default == nil
}

func (c *InputConfig) DefaultAsString() string {
	return getOrDefault(c.Default, reflect.String, "").(string)
}

func (c *InputConfig) DefaultAsBool() bool {
	return getOrDefault(c.Default, reflect.Bool, false).(bool)
}

func (c *InputConfig) DefaultAsInt() int {
	// Dirty work-around to avoid the conflicting two errors:
	// - panic: interface conversion: interface {} is int64, not int
	// - panic: interface conversion: interface {} is int, not int64
	var v int
	v64, is64b := getOrDefault(c.Default, reflect.Int, 0).(int64)
	if is64b {
		v = int(v64)
	} else {
		v = getOrDefault(c.Default, reflect.Int, 0).(int)
	}
	return v
}

func (c *InputConfig) DefaultAsArray() ([]interface{}, error) {
	v, ok := getOrDefault(c.Default, reflect.Slice, []interface{}{}).([]interface{})
	if !ok {
		return nil, fmt.Errorf("default value is not a slice: %v", c.Default)
	}
	return v, nil
}

func (c *InputConfig) DefaultAsObject() (map[string]interface{}, error) {
	if s, ok := c.Default.(string); ok {
		return sourceToObject(s)
	}

	v, ok := getOrDefault(c.Default, reflect.Map, map[string]interface{}{}).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("default value is not a map: %v", c.Default)
	}
	return v, nil
}

func (c *InputConfig) TypeName() string {
	var tpe string
	if c.Type == "" {
		tpe = "string"
	} else {
		tpe = c.Type
	}
	return tpe
}

func (c *InputConfig) JSONSchema() map[string]interface{} {
	jsonschema := map[string]interface{}{}
	if c.Properties != nil {
		jsonschema["properties"] = c.Properties
	}
	for k, v := range c.Remainings {
		jsonschema[k] = v
	}
	jsonschema["type"] = c.TypeName()
	return jsonschema
}

type ParameterConfig struct {
	Name        string                            `yaml:"name,omitempty"`
	Description string                            `yaml:"description,omitempty"`
	Type        string                            `yaml:"type,omitempty"`
	Default     interface{}                       `yaml:"default,omitempty"`
	Required    bool                              `yaml:"required,omitempty"`
	Properties  map[string]map[string]interface{} `yaml:"properties,omitempty"`
	Remainings  map[string]interface{}            `yaml:",inline"`
}

type OptionConfig struct {
	Name        string                            `yaml:"name,omitempty"`
	Description string                            `yaml:"description,omitempty"`
	Type        string                            `yaml:"type,omitempty"`
	Default     interface{}                       `yaml:"default,omitempty"`
	Required    bool                              `yaml:"required,omitempty"`
	Properties  map[string]map[string]interface{} `yaml:"properties,omitempty"`
	Remainings  map[string]interface{}            `yaml:",inline"`
}
