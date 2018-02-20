package steps

import (
	"fmt"
	"github.com/mumoshu/variant/api/flow"
	"github.com/mumoshu/variant/api/step"
)

type FlowStepLoader struct{}

func (l FlowStepLoader) LoadStep(stepConfig step.StepConfig, context step.LoadingContext) (step.Step, error) {
	if flowKey, isStr := stepConfig.Get("flow").(string); isStr && flowKey != "" {
		return FlowStep{
			Name:           stepConfig.GetName(),
			FlowKeyString:  flowKey,
			ProvidedInputs: flow.NewProvidedInputs(stepConfig.GetStringMapOrEmpty("inputs")),
		}, nil
	}

	return nil, fmt.Errorf("could'nt load flow step")
}

func NewFlowStepLoader() FlowStepLoader {
	return FlowStepLoader{}
}

type FlowStep struct {
	Name           string
	FlowKeyString  string
	ProvidedInputs flow.ProvidedInputs
}

func (s FlowStep) Run(context step.ExecutionContext) (step.StepStringOutput, error) {
	output, err := context.RunAnotherFlow(s.FlowKeyString, s.ProvidedInputs)
	return step.StepStringOutput{String: output}, err
}

func (s FlowStep) GetName() string {
	return s.Name
}
