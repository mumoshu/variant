package steps

import (
	"../engine"
)

type FlowStepLoader struct{}

func (l FlowStepLoader) TryToLoad(stepConfig engine.StepConfig) engine.Step {
	if flowKey, isStr := stepConfig.Flow.(string); isStr && flowKey != "" {
		return FlowStep{
			Name:          stepConfig.Name,
			FlowKeyString: flowKey,
		}
	}

	return nil
}

func NewFlowStepLoader() FlowStepLoader {
	return FlowStepLoader{}
}

type FlowStep struct {
	Name          string
	FlowKeyString string
}

func (s FlowStep) Run(project *engine.Application, flow *engine.BoundFlow, caller ...engine.Flow) (engine.StepStringOutput, error) {
	output, err := project.RunFlowForKeyString(s.FlowKeyString, []string{}, caller...)
	return engine.StepStringOutput{String: output}, err
}

func (s FlowStep) GetName() string {
	return s.Name
}
