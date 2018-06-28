package variant

import "strings"

type ResolvedInput struct {
	InputConfig
	TaskKey  TaskName
	FullName string
}

func (v *ResolvedInput) ShortName() string {
	return strings.SplitN(v.FullName, ".", 2)[1]
}
