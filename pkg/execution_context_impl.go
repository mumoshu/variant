package variant

import (
	"github.com/mumoshu/variant/pkg/api/step"
	"github.com/mumoshu/variant/pkg/api/task"
	"text/template"
)

type ExecutionContextImpl struct {
	app  Application
	task BoundTask
}

func NewExecutionContextImpl(app Application, task BoundTask) step.ExecutionContext {
	return ExecutionContextImpl{
		app:  app,
		task: task,
	}
}

func (c ExecutionContextImpl) GenerateAutoenv() (map[string]string, error) {
	return c.task.GenerateAutoenv()
}

func (c ExecutionContextImpl) Caller() []step.Caller {
	var caller step.Caller
	caller = c.task
	return []step.Caller{caller}
}

func (c ExecutionContextImpl) Key() step.Key {
	return c.task.Name
}

func (c ExecutionContextImpl) Vars() map[string]interface{} {
	return c.task.Vars
}

func (c ExecutionContextImpl) CreateFuncMap() template.FuncMap {
	return c.task.CreateFuncMap()
}

func (c ExecutionContextImpl) ProjectName() string {
	return c.task.ProjectName
}

func (c ExecutionContextImpl) Autoenv() bool {
	return c.task.Autoenv
}

func (c ExecutionContextImpl) Autodir() bool {
	return c.task.Autodir
}

func (c ExecutionContextImpl) Interactive() bool {
	return c.task.Interactive
}

func (c ExecutionContextImpl) RunAnotherTask(key string, provided task.ProvidedInputs) (string, error) {
	return c.app.RunTaskForKeyString(key, []string{}, provided, c.Caller()...)
}
