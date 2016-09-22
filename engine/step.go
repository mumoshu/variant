package engine

type Step interface {
	GetName() string
	Run(project *Project, flow *Flow, caller ...FlowDef) (StepStringOutput, error)
}

type StepStringOutput struct {
	String string
}
