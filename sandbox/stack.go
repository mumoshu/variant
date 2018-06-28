package sandbox

type Stack struct {
	tasksInStack []*Flow
}

func NewStackFromTasks(f ...*Flow) *Stack {
	return &Stack{
		tasksInStack: f,
	}
}

func (s *Stack) Pop() (*Stack, *Flow) {
	return &Stack{
		tasksInStack: s.tasksInStack[0 : len(s.tasksInStack)-1],
	}, s.tasksInStack[len(s.tasksInStack)-1]
}

func (s *Stack) Push(flow *Flow) *Stack {
	return &Stack{
		tasksInStack: append(append([]*Flow{}, s.tasksInStack...), flow),
	}
}

func (s *Stack) Size() int {
	return len(s.tasksInStack)
}

func (s *Stack) PushMulti(tasks []*Flow) *Stack {
	state := append([]*Flow{}, s.tasksInStack...)

	return &Stack{
		tasksInStack: append(state, tasks...),
	}
}

func (s *Stack) Concat(other *Stack) *Stack {
	return &Stack{
		tasksInStack: append(append([]*Flow{}, s.tasksInStack...), other.tasksInStack...),
	}
}

func (s *Stack) FromBottom() []*Flow {
	return s.tasksInStack
}

func (s *Stack) Top() *Flow {
	return s.tasksInStack[len(s.tasksInStack)-1]
}
