package variant

import (
	"github.com/juju/errors"
	"github.com/mumoshu/variant/pkg/api/step"
	"strings"
)

type TaskName struct {
	Components []string
}

func (t TaskName) Simple() string {
	return t.Components[len(t.Components)-1]
}

func (t TaskName) String() string {
	return strings.Join(t.Components, ".")
}

func (t TaskName) ShortString() string {
	return strings.Join(t.Components[1:], ".")
}

func (t TaskName) Parent() (step.Key, error) {
	if len(t.Components) > 1 {
		return TaskName{Components: t.Components[:len(t.Components)-1]}, nil
	} else {
		return TaskName{}, errors.Errorf("TaskName %v doesn't have a parent", t)
	}
}
