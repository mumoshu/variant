package variant

type Key interface {
	ShortString() string
	Parent() (Key, error)
}

type Caller interface {
	GetKey() Key
}

type LoadingContext interface {
	LoadStep(config StepDef) (Step, error)
}

type Step interface {
	GetName() string
	Run(context ExecutionContext) (StepStringOutput, error)
	Silent() bool
}

type StepStringOutput struct {
	String string
}
