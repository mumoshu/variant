package variant

import (
	"github.com/mumoshu/variant/pkg/api/step"
	"github.com/mumoshu/variant/pkg/api/task"
)

type stepExecContext struct {
	app          Application
	taskRunner   TaskRunner
	taskTemplate *TaskTemplate
}

func NewStepExecutionContext(app Application, taskRunner TaskRunner, taskTemplate *TaskTemplate) step.ExecutionContext {
	return stepExecContext{
		app:          app,
		taskRunner:   taskRunner,
		taskTemplate: taskTemplate,
	}
}

func (c stepExecContext) GenerateAutoenv() (map[string]string, error) {
	return c.taskRunner.GenerateAutoenv()
}

func (c stepExecContext) Caller() []step.Caller {
	return []step.Caller{c.taskRunner.AsStepCaller()}
}

func (c stepExecContext) Key() step.Key {
	return c.taskRunner.Name.AsStepKey()
}

func (c stepExecContext) Vars() map[string]interface{} {
	return c.taskRunner.Values
}

func (c stepExecContext) Render(expr string, name string) (string, error) {
	return c.taskTemplate.Render(expr, name)
}

func (c stepExecContext) Autoenv() bool {
	return c.taskRunner.Autoenv
}

func (c stepExecContext) Autodir() bool {
	return c.taskRunner.Autodir
}

func (c stepExecContext) Interactive() bool {
	return c.taskRunner.Interactive
}

func (c stepExecContext) RunAnotherTask(key string, arguments task.Arguments, scope map[string]interface{}) (string, error) {
	return c.app.RunTaskForKeyString(key, []string{}, arguments, scope, c.taskRunner.Task)
}
