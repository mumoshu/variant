package variant

import (
	"github.com/pkg/errors"
)

type TaskRegistry struct {
	tasks map[string]*Task
}

func NewTaskRegistry() *TaskRegistry {
	return &TaskRegistry{
		tasks: map[string]*Task{},
	}
}

func (p *TaskRegistry) Tasks() map[string]*Task {
	return p.tasks
}

func (p *TaskRegistry) FindTask(name TaskName) (*Task, error) {
	t := p.tasks[name.ShortString()]

	if t == nil {
		return nil, errors.Errorf("No Task exists for the task name `%s`", name.ShortString())
	}

	return t, nil
}

func (p *TaskRegistry) put(key TaskName, task *Task) {
	p.tasks[key.ShortString()] = task
}

func (p *TaskRegistry) RegisterTasks(task *Task) {
	p.put(task.Name, task)

	for _, child := range task.Tasks {
		p.RegisterTasks(child)
	}
}

func (p *TaskRegistry) AllTaskKeys() []string {
	allTasks := []string{}
	for _, t := range p.tasks {
		allTasks = append(allTasks, t.Name.String())
	}
	return allTasks
}
