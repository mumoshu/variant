package variant

import (
	"github.com/spf13/cobra"
)

type Task struct {
	TaskDef
	Name           TaskName
	ProjectName    string
	ResolvedInputs []*Input
	Tasks          []*Task
	Command        *cobra.Command
}

func (f Task) GetKey() TaskName {
	return f.Name
}
