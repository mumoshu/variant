package engine

import (
	"github.com/juju/errors"
	"github.com/mumoshu/variant/api/step"
	"strings"
)

type FlowKey struct {
	Components []string
}

func (t FlowKey) String() string {
	return strings.Join(t.Components, ".")
}

func (t FlowKey) ShortString() string {
	return strings.Join(t.Components[1:], ".")
}

func (t FlowKey) Parent() (step.Key, error) {
	if len(t.Components) > 1 {
		return FlowKey{Components: t.Components[:len(t.Components)-1]}, nil
	} else {
		return FlowKey{}, errors.Errorf("FlowKey %v doesn't have a parent", t)
	}
}
