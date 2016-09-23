package engine

import (
	"github.com/spf13/cobra"
)

type Flow struct {
	Key            FlowKey
	ProjectName    string
	Steps          []Step
	Inputs         []*Input
	ResolvedInputs []*ResolvedInput
	Autoenv        bool
	Autodir        bool
	Interactive    bool
	FlowConfig     *FlowConfig
	Command        *cobra.Command
}
