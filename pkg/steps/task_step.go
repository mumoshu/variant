package steps

import (
	"fmt"
	"github.com/mumoshu/variant/pkg/api/step"
	"github.com/mumoshu/variant/pkg/api/task"
)

type TaskStepLoader struct{}

func (l TaskStepLoader) LoadStep(stepConfig step.StepDef, context step.LoadingContext) (step.Step, error) {
	if taskKey, isStr := stepConfig.Get("task").(string); isStr && taskKey != "" {
		inputs := task.NewArguments(stepConfig.GetStringMapOrEmpty("inputs"))
		if len(inputs) == 0 {
			inputs = task.NewArguments(stepConfig.GetStringMapOrEmpty("arguments"))
		}

		return TaskStep{
			Name:           stepConfig.GetName(),
			TaskKeyString:  taskKey,
			ProvidedInputs: inputs,
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
	ProvidedInputs task.Arguments
}

func (s TaskStep) Run(context step.ExecutionContext) (step.StepStringOutput, error) {
	output, err := context.RunAnotherTask(s.TaskKeyString, s.ProvidedInputs)
	return step.StepStringOutput{String: output}, err
}

func (s TaskStep) GetName() string {
	return s.Name
}
