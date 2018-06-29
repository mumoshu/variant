package variant

import (
	"github.com/juju/errors"
	"strings"
)

type TaskCreator struct {
	taskNamer *TaskNamer
}

func NewTaskCreator(c *TaskNamer) *TaskCreator {
	return &TaskCreator{
		taskNamer: c,
	}
}

func (g *TaskCreator) Create(taskDef *TaskDef, parentTaskNameComponents []string, appName string) (*Task, error) {
	taskNameComponents := append(parentTaskNameComponents, taskDef.Name)
	taskNameStr := strings.Join(taskNameComponents, ".")
	taskName := g.taskNamer.FromString(taskNameStr)
	task := &Task{
		Name:        taskName,
		ProjectName: appName,
		//Command:     cmd,
		TaskDef: *taskDef,
	}

	subTasks := []*Task{}

	for _, c := range task.TaskDefs {
		f, err := g.Create(c, taskNameComponents, appName)

		if err != nil {
			return nil, errors.Trace(err)
		}

		subTasks = append(subTasks, f)
	}

	task.Tasks = subTasks

	return task, nil
}
