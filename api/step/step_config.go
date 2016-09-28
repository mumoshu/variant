package step

import (
	"github.com/Sirupsen/logrus"
	"reflect"
)

type StepConfigImpl struct {
	raw map[string]interface{}
}

type StepConfig interface {
	GetName() string
	Raw() map[string]interface{}
	Get(key string) interface{}
}

func (c StepConfigImpl) GetName() string {
	str, ok := c.raw["name"].(string)

	if !ok {
		logrus.Panicf("name wasn't string! name=%s raw=%v", reflect.TypeOf(c.raw["name"]), c.raw)
	}

	return str
}

func (c StepConfigImpl) Raw() map[string]interface{} {
	return c.raw
}

func (c StepConfigImpl) Get(key string) interface{} {
	return c.raw[key]
}

func NewStepConfig(raw map[string]interface{}) StepConfig {
	return StepConfigImpl{
		raw: raw,
	}
}
