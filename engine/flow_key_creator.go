package engine

import "strings"

type FlowKeyCreator struct {
	AppName string
}

func NewFlowKeyCreator(appName string) *FlowKeyCreator {
	return &FlowKeyCreator{
		AppName: appName,
	}
}

func (p FlowKeyCreator) CreateFlowKey(flowKeyStr string) FlowKey {
	c := strings.Split(flowKeyStr, ".")
	return FlowKey{Components: c}
}

func (p FlowKeyCreator) CreateFlowKeyFromResolvedInput(variable *ResolvedInput) FlowKey {
	return p.CreateFlowKeyFromInputName(variable.Name)
}

func (p FlowKeyCreator) CreateFlowKeyFromInput(input *Input) FlowKey {
	return p.CreateFlowKeyFromInputName(input.Name)
}

func (p FlowKeyCreator) CreateFlowKeyFromInputName(inputName string) FlowKey {
	c := strings.Split(p.AppName+"."+inputName, ".")
	return FlowKey{Components: c}
}
