package sandbox

import (
	"fmt"
	"strings"
)

type ScopedFlow struct {
	Stack *Stack
}

func NewScopedFlow(stack *Stack) *ScopedFlow {
	return &ScopedFlow{
		Stack: stack,
	}
}

func (e ScopedFlow) GetName() string {
	return e.Current().Name
}

func (e ScopedFlow) Current() *Flow {
	return e.Stack.Top()
}

func (e ScopedFlow) Path() string {
	flowNames := []string{}

	for _, flow := range e.Stack.FromBottom() {
		flowNames = append(flowNames, flow.Name)
	}

	return strings.Join(flowNames, ".")
}

func (e ScopedFlow) Scope() *Scope {
	result := []*ScopedFlow{}
	stack := e.Stack
	count := e.Stack.Size()

	for i := 0; i < count; i++ {
		expr := NewScopedFlow(stack)
		result = append(result, expr)
		stack, _ = stack.Pop()
	}
	return NewScope(result)
}

func (e ScopedFlow) ScopeInString() string {
	return e.Scope().String()
}

func (e ScopedFlow) String() string {
	stack := []string{}
	for _, f := range e.Stack.FromBottom() {
		stack = append(stack, f.Name)
	}
	return fmt.Sprintf("FlowInContext(Name=%s,Path=%s,Stack=%s,Scope=%s)", e.GetName(), e.Path(), strings.Join(stack, "<-"), e.ScopeInString())
}

func (e ScopedFlow) FindFlowByPathComponents(components []string) (*ScopedFlow, error) {
	stack, err := e.buildStackUntil(components)

	if err != nil {
		return nil, err
	}

	if stack.Size() == 0 {
		panic("A stack is zero-length! Possibly a bug!")
	}

	nameOfCurrentFlow := stack.Top().Name
	lastComponent := components[len(components)-1]

	if nameOfCurrentFlow != lastComponent {
		panic(fmt.Sprintf("Names(%s and %s) didn't match! Possibly a bug!", nameOfCurrentFlow, lastComponent))
	}

	newStack, _ := e.Stack.Pop()

	expr := NewScopedFlow(newStack.Concat(stack))

	if expr.GetName() != expr.Stack.Top().Name {
		panic("BUG!")
	}

	return expr, nil
}

func (e ScopedFlow) buildStackUntil(components []string) (*Stack, error) {
	return e.Current().BuildStackUntil(components)
}
