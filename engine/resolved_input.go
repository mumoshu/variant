package engine

import "strings"

type ResolvedInput struct {
	Input
	FlowKey  FlowKey
	FullName string
}

func (v *ResolvedInput) ShortName() string {
	return strings.SplitN(v.FullName, ".", 2)[1]
}
