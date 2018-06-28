package steps

import (
	"fmt"
	"github.com/mumoshu/variant/api/step"
	"github.com/mumoshu/variant/api/task"
)

type TaskStepLoader struct{}

func (l TaskStepLoader) LoadStep(stepConfig step.StepConfig, context step.LoadingContext) (step.Step, error) {
	if taskKey, isStr := stepConfig.Get("task").(string); isStr && taskKey != "" {
		return TaskStep{
			Name:           stepConfig.GetName(),
			TaskKeyString:  taskKey,
			ProvidedInputs: task.NewProvidedInputs(stepConfig.GetStringMapOrEmpty("inputs")),
		}, nil
	}

	return nil, fmt.Errorf("could'nt load task step")
}

func NewTaskStepLoader() TaskStepLoader {
	return TaskStepLoader{}
}

type TaskStep struct {
	Name           string
	TaskKeyString  string
	ProvidedInputs task.ProvidedInputs
}

func (s TaskStep) Run(context step.ExecutionContext) (step.StepStringOutput, error) {
	output, err := context.RunAnotherTask(s.TaskKeyString, s.ProvidedInputs)
	return step.StepStringOutput{String: output}, err
}

func (s TaskStep) GetName() string {
	return s.Name
}
