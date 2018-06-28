package variant

import (
	"github.com/juju/errors"
	"strings"
)

type TaskGenerator struct {
	taskNamer *TaskNamer
}

func NewTaskGenerator(c *TaskNamer) *TaskGenerator {
	return &TaskGenerator{
		taskNamer: c,
	}
}

func (g *TaskGenerator) GenerateTask(taskConfig *TaskConfig, parentTaskNameComponents []string, appName string) (*Task, error) {
	taskNameComponents := append(parentTaskNameComponents, taskConfig.Name)
	taskNameStr := strings.Join(taskNameComponents, ".")
	taskName := g.taskNamer.FromString(taskNameStr)
	task := &Task{
		Name:        taskName,
		ProjectName: appName,
		//Command:     cmd,
		TaskConfig: *taskConfig,
	}

	tasks := []*Task{}

	for _, c := range task.TaskConfigs {
		f, err := g.GenerateTask(c, taskNameComponents, appName)

		if err != nil {
			return nil, errors.Trace(err)
		}

		tasks = append(tasks, f)
	}

	task.Tasks = tasks

	return task, nil
}
