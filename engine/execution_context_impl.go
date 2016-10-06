package engine

import (
	"../api/flow"
	"../api/step"
	"text/template"
)

type ExecutionContextImpl struct {
	app  Application
	flow BoundFlow
}

func NewExecutionContextImpl(app Application, flow BoundFlow) step.ExecutionContext {
	return ExecutionContextImpl{
		app:  app,
		flow: flow,
	}
}

func (c ExecutionContextImpl) GenerateAutoenv() (map[string]string, error) {
	return c.flow.GenerateAutoenv()
}

func (c ExecutionContextImpl) Caller() []step.Caller {
	var caller step.Caller
	caller = c.flow
	return []step.Caller{caller}
}

func (c ExecutionContextImpl) Key() step.Key {
	return c.flow.Key
}

func (c ExecutionContextImpl) Vars() map[string]interface{} {
	return c.flow.Vars
}

func (c ExecutionContextImpl) CreateFuncMap() template.FuncMap {
	return c.flow.CreateFuncMap()
}

func (c ExecutionContextImpl) ProjectName() string {
	return c.flow.ProjectName
}

func (c ExecutionContextImpl) Autoenv() bool {
	return c.flow.Autoenv
}

func (c ExecutionContextImpl) Autodir() bool {
	return c.flow.Autodir
}

func (c ExecutionContextImpl) Interactive() bool {
	return c.flow.Interactive
}

func (c ExecutionContextImpl) RunAnotherFlow(key string, provided flow.ProvidedInputs) (string, error) {
	return c.app.RunFlowForKeyString(key, []string{}, provided, c.Caller()...)
}
