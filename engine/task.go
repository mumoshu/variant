package engine

import (
	"github.com/mumoshu/variant/api/step"
	"github.com/spf13/cobra"
)

type Task struct {
	TaskConfig
	Name           TaskName
	ProjectName    string
	ResolvedInputs []*ResolvedInput
	Tasks          []*Task
	Command        *cobra.Command
}

func (f Task) GetKey() step.Key {
	return f.Name
}
