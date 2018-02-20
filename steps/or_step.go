package steps

import (
	"fmt"
	"github.com/juju/errors"
	"github.com/mumoshu/variant/api/step"
	"github.com/mumoshu/variant/util/maputil"
	"log"
	"reflect"
)

type OrStepLoader struct{}

func (l OrStepLoader) LoadStep(config step.StepConfig, context step.LoadingContext) (step.Step, error) {
	data := config.Get("or")

	if data == nil {
		return nil, fmt.Errorf("no field named or exists, config=%v", config)
	}

	steps, ok := data.([]interface{})

	if !ok {
		return nil, fmt.Errorf("field \"or\" must be a map but it wasn't: %v", data)
	}

	result := OrStep{
		Name:  config.GetName(),
		Steps: []step.Step{},
	}

	for i, s := range steps {
		stepAsMap, isMap := s.(map[interface{}]interface{})

		if !isMap {
			log.Panicf("isnt map! %s", reflect.TypeOf(s))
		}

		converted, conversionErr := maputil.CastKeysToStrings(stepAsMap)
		if conversionErr != nil {
			return nil, errors.Trace(conversionErr)
		}

		if converted["name"] == "" || converted["name"] == nil {
			converted["name"] = fmt.Sprintf("or[%d]", i)
		}

		step, loadingErr := context.LoadStep(step.NewStepConfig(converted))
		if loadingErr != nil {
			return nil, errors.Trace(loadingErr)
		}

		result.Steps = append(result.Steps, step)
	}

	return result, nil
}

func NewOrStepLoader() OrStepLoader {
	return OrStepLoader{}
}

type OrStep struct {
	Name  string
	Steps []step.Step
}

func (s OrStep) Run(context step.ExecutionContext) (step.StepStringOutput, error) {
	var lastError error
	for _, s := range s.Steps {
		var output step.StepStringOutput

		output, lastError = s.Run(context)

		if lastError == nil {
			return output, nil
		}
	}
	return step.StepStringOutput{String: "all steps failed"}, errors.Annotatef(lastError, "all steps failed")
}

func (s OrStep) GetName() string {
	return s.Name
}
