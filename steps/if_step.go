package steps

import (
	"../api/step"
	"../util/maputil"
	"fmt"
	"github.com/juju/errors"
	"log"
	"reflect"
)

type IfStepLoader struct{}

func (l IfStepLoader) LoadStep(config step.StepConfig, context step.LoadingContext) (step.Step, error) {
	ifData := config.Get("if")

	if ifData == nil {
		return nil, fmt.Errorf("no field named `if` exists, config=%v", config)
	}

	ifArray, ok := ifData.(interface{})

	if !ok {
		return nil, fmt.Errorf("field \"if\" must be an interface{} but it wasn't: %v", ifData)
	}

	thenData := config.Get("then")

	if thenData == nil {
		return nil, fmt.Errorf("no field named `then` exists, config=%v", config)
	}

	thenArray, ok2 := thenData.(interface{})

	if !ok2 {
		return nil, fmt.Errorf("field \"then\" must be an interface{} but it wasn't: %v", ifData)
	}

	result := IfStep{
		Name: config.GetName(),
		If:   []step.Step{},
		Then: []step.Step{},
	}

	ifInput, ifErr := readSteps(ifArray, context)

	if ifErr != nil {
		return nil, errors.Annotatef(ifErr, "reading `if` failed")
	}

	thenInput, thenErr := readSteps(thenArray, context)

	if thenErr != nil {
		return nil, errors.Annotatef(thenErr, "reading `then` failed")
	}

	result.If = ifInput
	result.Then = thenInput

	return result, nil
}

func readSteps(input interface{}, context step.LoadingContext) ([]step.Step, error) {
	steps, ok := input.([]interface{})

	if !ok {
		return nil, fmt.Errorf("input must be array: input=%v", input)
	}

	result := []step.Step{}

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

		result = append(result, step)
	}

	return result, nil
}

func NewIfStepLoader() IfStepLoader {
	return IfStepLoader{}
}

type IfStep struct {
	Name string
	If   []step.Step
	Then []step.Step
}

func run(steps []step.Step, context step.ExecutionContext) (step.StepStringOutput, error) {
	var lastOutput step.StepStringOutput
	var lastError error

	for _, s := range steps {
		lastOutput, lastError = s.Run(context)

		if lastError != nil {
			return step.StepStringOutput{String: "run error"}, errors.Annotatef(lastError, "failed running step")
		}
	}

	return lastOutput, nil
}

func (s IfStep) Run(context step.ExecutionContext) (step.StepStringOutput, error) {
	_, ifErr := run(s.If, context)

	if ifErr != nil {
		return step.StepStringOutput{String: "if step failed"}, nil
	}

	thenOut, thenErr := run(s.Then, context)

	if thenErr != nil {
		return step.StepStringOutput{String: "then step failed"}, errors.Annotatef(thenErr, "`then` steps failed")
	}

	return thenOut, nil
}

func (s IfStep) GetName() string {
	return s.Name
}
