package variant

import "strings"

type Input struct {
	InputConfig
	TaskKey  TaskName
	FullName string
}

func (v *Input) ShortName() string {
	return strings.SplitN(v.FullName, ".", 2)[1]
}
