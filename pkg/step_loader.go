package variant

type StepLoader interface {
	LoadStep(config StepDef, context LoadingContext) (Step, error)
}
