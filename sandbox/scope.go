package sandbox

import (
	"os"
	"fmt"
	"strings"
)

type Scope struct {
	scopedFlowsInStack []*ScopedFlow
}

func NewScope(flowsInScope []*ScopedFlow) *Scope {
	return &Scope {
		scopedFlowsInStack: flowsInScope,
	}
}

func (s *Scope) String() string {
	result := []string{}
	for _, flow := range s.scopedFlowsInStack {
		str := flow.Path()
		result = append(result, str)
	}
	return strings.Join(result, ",")
}

func (s Scope) FindFlowAtPath(path string) (*ScopedFlow, error) {
	e := s.scopedFlowsInStack[len(s.scopedFlowsInStack)-1]

	os.Stderr.WriteString(fmt.Sprintf("%s -> %s\n", e.Path(), path))

	for _, scoped := range s.scopedFlowsInStack {
		if found, err := scoped.FindFlowByPathComponents(strings.Split(path, ".")); err == nil {
			os.Stderr.WriteString(fmt.Sprintf("  %s -> %s [found %s]\n", scoped.Path(), path, found))
			return found, nil
		} else {
			os.Stderr.WriteString(fmt.Sprintf("  %s -> %s [not found]\n", scoped.Path(), path))
		}
	}
	return nil, fmt.Errorf("No expr found at path \"%s\"", path)
}
