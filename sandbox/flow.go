package sandbox

import (
	"fmt"
	"strings"
)

type Input struct {
	Name string `yaml:"name,omitempty"`
}

type Flow struct {
	Name   string  `yaml:"name,omitempty"`
	Inputs []Input `yaml:"inputs,omitempty"`
	// i.e. symbol table
	Flows  []Flow            `yaml:"flows,omitempty"`
	Script string            `yaml:"script,omitempty"`
	Env    map[string]string `yaml:"env,omitempty"`
}

type Resolver interface {
	GetName() string
	FindExprAtPath(path string) (*ScopedFlow, error)
}

type FlowRunParams struct {
	Caller *Flow
}

type FlowRun struct {
	Target Flow
	Params FlowRunParams
}

type StepRunContext struct {
	FlowRun FlowRun
}

type StepRunParams struct {
	Context StepRunContext
}

func (f Flow) GetName() string {
	return f.Name
}

func (f Flow) AsRoot() *ScopedFlow {
	return NewScopedFlow(NewStackFromFlows(&f))
}

func (f Flow) FindExprAtPath(path string) (*ScopedFlow, error) {
	return f.FindFlowByPathComponents(strings.Split(path, "."))
}

func (f Flow) FindFlowByPathComponents(components []string) (*ScopedFlow, error) {
	stack, err := f.BuildStackUntil(components)

	if err != nil {
		return nil, err
	}

	flow := NewScopedFlow(stack)

	return flow, nil
}

func (f Flow) BuildStackUntil(components []string) (*Stack, error) {
	current := NewStackFromFlows(&f)

	if len(components) == 0 {
		panic("Path components are empty. Possibly a bug!")
	}

	targetChildName := components[0]

	for _, next := range f.Flows {
		if next.Name == targetChildName {
			if len(components) == 1 {
				return current.Push(&next), nil
			} else {
				rest, err := next.BuildStackUntil(components[1:])

				if err != nil {
					return nil, err
				}

				return current.Concat(rest), nil
			}
		}
	}

	if f.Name == targetChildName {
		return current, nil
	}

	return nil, fmt.Errorf("No flow named %s found", strings.Join(components, "."))
}
