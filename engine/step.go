package engine

type Step interface {
	GetName() string
	Run(project *Application, flow *BoundFlow, caller ...Flow) (StepStringOutput, error)
}

type StepStringOutput struct {
	String string
}
