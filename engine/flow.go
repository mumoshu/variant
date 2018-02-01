package engine

import (
	"github.com/mumoshu/variant/api/step"
	"github.com/spf13/cobra"
)

type Flow struct {
	FlowConfig
	Key            FlowKey
	ProjectName    string
	ResolvedInputs []*ResolvedInput
	Flows          []*Flow
	Command        *cobra.Command
}

func (f Flow) GetKey() step.Key {
	return f.Key
}
