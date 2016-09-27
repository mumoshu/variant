package sandbox

type Stack struct {
	flowsInStack []*Flow
}

func NewStackFromFlows(f ...*Flow) *Stack {
	return &Stack{
		flowsInStack: f,
	}
}

func (s *Stack) Pop() (*Stack, *Flow) {
	return &Stack{
		flowsInStack: s.flowsInStack[0 : len(s.flowsInStack)-1],
	}, s.flowsInStack[len(s.flowsInStack)-1]
}

func (s *Stack) Push(flow *Flow) *Stack {
	return &Stack{
		flowsInStack: append(append([]*Flow{}, s.flowsInStack...), flow),
	}
}

func (s *Stack) Size() int {
	return len(s.flowsInStack)
}

func (s *Stack) PushMulti(flows []*Flow) *Stack {
	state := append([]*Flow{}, s.flowsInStack...)

	return &Stack{
		flowsInStack: append(state, flows...),
	}
}

func (s *Stack) Concat(other *Stack) *Stack {
	return &Stack{
		flowsInStack: append(append([]*Flow{}, s.flowsInStack...), other.flowsInStack...),
	}
}

func (s *Stack) FromBottom() []*Flow {
	return s.flowsInStack
}

func (s *Stack) Top() *Flow {
	return s.flowsInStack[len(s.flowsInStack)-1]
}
