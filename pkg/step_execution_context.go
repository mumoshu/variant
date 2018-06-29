package variant

import (
	"github.com/mumoshu/variant/pkg/api/step"
	"github.com/mumoshu/variant/pkg/api/task"
	"text/template"
)

type stepExecContext struct {
	app  Application
	task TaskRunner
}

func NewStepExecutionContext(app Application, task TaskRunner) step.ExecutionContext {
	return stepExecContext{
		app:  app,
		task: task,
	}
}

func (c stepExecContext) GenerateAutoenv() (map[string]string, error) {
	return c.task.GenerateAutoenv()
}

func (c stepExecContext) Caller() []step.Caller {
	return []step.Caller{c.task.AsStepCaller()}
}

func (c stepExecContext) Key() step.Key {
	return c.task.Name.AsStepKey()
}

func (c stepExecContext) Vars() map[string]interface{} {
	return c.task.Values
}

func (c stepExecContext) CreateFuncMap() template.FuncMap {
	return c.task.CreateFuncMap()
}

func (c stepExecContext) ProjectName() string {
	return c.task.ProjectName
}

func (c stepExecContext) Autoenv() bool {
	return c.task.Autoenv
}

func (c stepExecContext) Autodir() bool {
	return c.task.Autodir
}

func (c stepExecContext) Interactive() bool {
	return c.task.Interactive
}

func (c stepExecContext) RunAnotherTask(key string, provided task.Arguments) (string, error) {
	return c.app.RunTaskForKeyString(key, []string{}, provided, c.task.Task)
}
