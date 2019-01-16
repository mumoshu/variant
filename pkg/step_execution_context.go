package variant

import (
	"github.com/mumoshu/variant/pkg/api/task"
)

type ExecutionContext struct {
	app          Application
	taskRunner   TaskRunner
	taskTemplate *TaskTemplate
	trace        []*Task
	asInput      bool
}

func NewStepExecutionContext(app Application, taskRunner TaskRunner, taskTemplate *TaskTemplate, asInput bool, trace []*Task) ExecutionContext {
	return ExecutionContext{
		app:          app,
		taskRunner:   taskRunner,
		taskTemplate: taskTemplate,
		trace:        trace,
		asInput:      asInput,
	}
}

func (c ExecutionContext) GenerateAutoenv() (map[string]string, error) {
	return c.taskRunner.GenerateAutoenv()
}

func (c ExecutionContext) Caller() []Caller {
	return []Caller{c.taskRunner.AsStepCaller()}
}

func (c ExecutionContext) Key() Key {
	return c.taskRunner.Name.AsStepKey()
}

func (c ExecutionContext) Vars() map[string]interface{} {
	return c.taskRunner.Values
}

func (c ExecutionContext) Render(expr string, name string) (string, error) {
	return c.taskTemplate.Render(expr, name)
}

func (c ExecutionContext) Autoenv() bool {
	return c.taskRunner.Autoenv
}

func (c ExecutionContext) Autodir() bool {
	return c.taskRunner.Autodir
}

func (c ExecutionContext) Interactive() bool {
	return c.taskRunner.Interactive
}

func (c ExecutionContext) RunAnotherTask(key string, arguments task.Arguments, scope map[string]interface{}) (string, error) {
	return c.app.RunTaskForKeyString(key, []string{}, arguments, scope, c.asInput, c.taskRunner.Task)
}
