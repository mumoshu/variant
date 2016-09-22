package engine

type Step interface {
	GetName() string
	Run(project *Project, flow *Flow, parent ...FlowDef) (StepStringOutput, error)
}

type StepStringOutput struct {
	String string
}
