package variant

import (
	"github.com/juju/errors"
	"github.com/mumoshu/variant/pkg/api/step"
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

func (p *TaskRegistry) FindTask(flowKey step.Key) (*Task, error) {
	t := p.tasks[flowKey.ShortString()]

	if t == nil {
		return nil, errors.Errorf("No Task exists for the task key `%s`", flowKey.ShortString())
	}

	return t, nil
}

func (p *TaskRegistry) RegisterTask(flowKey step.Key, flowDef *Task) {
	p.tasks[flowKey.ShortString()] = flowDef
}

func (p *TaskRegistry) RegisterTasks(flow *Task) {
	p.RegisterTask(flow.Name, flow)

	for _, child := range flow.Tasks {
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
