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
			Name:          stepConfig.GetName(),
			TaskKeyString: taskKey,
			Arguments:     inputs,
			silent:        stepConfig.Silent(),
		}, nil
	}

	return nil, fmt.Errorf("could'nt load task step")
}

func NewTaskStepLoader() TaskStepLoader {
	return TaskStepLoader{}
}

type TaskStep struct {
	Name          string
	TaskKeyString string
	Arguments     task.Arguments
	silent        bool
}

func (s TaskStep) Run(context step.ExecutionContext) (step.StepStringOutput, error) {
	output, err := context.RunAnotherTask(s.TaskKeyString, s.Arguments.TransformStringValues(func(v string) string {
		v2, err := context.Render(v, "argument")
		if err != nil {
			panic(err)
		}
		return v2
	}), context.Vars())
	return step.StepStringOutput{String: output}, err
}

func (s TaskStep) GetName() string {
	return s.Name
}

func (s TaskStep) Silent() bool {
	return s.silent
}
