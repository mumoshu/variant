package sandbox

import (
	"fmt"
	"os"
	"strings"
)

type Scope struct {
	scopedTasksInStack []*ScopedFlow
}

func NewScope(tasksInScope []*ScopedFlow) *Scope {
	return &Scope{
		scopedTasksInStack: tasksInScope,
	}
}

func (s *Scope) String() string {
	result := []string{}
	for _, flow := range s.scopedTasksInStack {
		str := flow.Path()
		result = append(result, str)
	}
	return strings.Join(result, ",")
}

func (s Scope) FindFlowAtPath(path string) (*ScopedFlow, error) {
	e := s.scopedTasksInStack[len(s.scopedTasksInStack)-1]

	os.Stderr.WriteString(fmt.Sprintf("%s -> %s\n", e.Path(), path))

	for _, scoped := range s.scopedTasksInStack {
		if found, err := scoped.FindFlowByPathComponents(strings.Split(path, ".")); err == nil {
			os.Stderr.WriteString(fmt.Sprintf("  %s -> %s [found %s]\n", scoped.Path(), path, found))
			return found, nil
		} else {
			os.Stderr.WriteString(fmt.Sprintf("  %s -> %s [not found]\n", scoped.Path(), path))
		}
	}
	return nil, fmt.Errorf("No expr found at path \"%s\"", path)
}
