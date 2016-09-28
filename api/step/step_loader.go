package step

type StepLoader interface {
	LoadStep(config StepConfig, context LoadingContext) (Step, error)
}
