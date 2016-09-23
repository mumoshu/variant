package engine

import (
	"github.com/spf13/cobra"
)

type Flow struct {
	FlowConfig
	Key            FlowKey
	ProjectName    string
	ResolvedInputs []*ResolvedInput
	Command        *cobra.Command
}
