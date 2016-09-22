package engine

import "strings"

type Variable struct {
	FlowKey     FlowKey
	FullName    string
	Name        string
	Parameters  map[string]Parameter
	Description string
	Candidates  []string
	Complete    string
}

func (v *Variable) ShortName() string {
	return strings.SplitN(v.FullName, ".", 2)[1]
}
