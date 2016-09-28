package steps

import (
	"../api/step"
	"fmt"
)

type FlowStepLoader struct{}

func (l FlowStepLoader) LoadStep(stepConfig step.StepConfig, context step.LoadingContext) (step.Step, error) {
	if flowKey, isStr := stepConfig.Get("flow").(string); isStr && flowKey != "" {
		return FlowStep{
			Name:          stepConfig.GetName(),
			FlowKeyString: flowKey,
		}, nil
	}

	return nil, fmt.Errorf("could'nt load flow step")
}

func NewFlowStepLoader() FlowStepLoader {
	return FlowStepLoader{}
}

type FlowStep struct {
	Name          string
	FlowKeyString string
}

func (s FlowStep) Run(context step.ExecutionContext) (step.StepStringOutput, error) {
	output, err := context.RunAnotherFlow(s.FlowKeyString)
	return step.StepStringOutput{String: output}, err
}

func (s FlowStep) GetName() string {
	return s.Name
}
