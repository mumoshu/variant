package engine

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
)

type InputResolver interface {
	ResolveInputs()
	ResolveInputsForFlow(flowDef *Flow) []*ResolvedInput
	ResolveInputsForFlowKey(currentFlowKey FlowKey, path string) []*ResolvedInput
}

type RegistryBasedInputResolver struct {
	InputResolver
	registry       *FlowRegistry
	flowKeyCreator *FlowKeyCreator
}

func NewRegistryBasedInputResolver(registry *FlowRegistry, flowKeyCreator *FlowKeyCreator) InputResolver {
	return &RegistryBasedInputResolver{
		registry:       registry,
		flowKeyCreator: flowKeyCreator,
	}
}

func (r *RegistryBasedInputResolver) ResolveInputs() {
	for _, flow := range r.registry.Flows() {
		flow.ResolvedInputs = r.ResolveInputsForFlow(flow)
	}
}

func (r *RegistryBasedInputResolver) ResolveInputsForFlow(flowDef *Flow) []*ResolvedInput {
	return r.ResolveInputsForFlowKey(flowDef.Key, "")
}

func (r *RegistryBasedInputResolver) ResolveInputsForFlowKey(currentFlowKey FlowKey, path string) []*ResolvedInput {
	result := []*ResolvedInput{}

	ctx := log.WithFields(log.Fields{"prefix": fmt.Sprintf("%s", currentFlowKey.String())})

	currentFlow, err := r.registry.FindFlow(currentFlowKey)

	if err != nil {
		allFlows := r.registry.AllFlowKeys()
		ctx.Debugf("is not a Flow in: %v", allFlows)
		return []*ResolvedInput{}
	}

	for _, input := range currentFlow.Inputs {
		childKey := r.flowKeyCreator.CreateFlowKeyFromInput(input)

		ctx.Debugf("depends on %s", childKey.String())

		vars := r.ResolveInputsForFlowKey(childKey, fmt.Sprintf("%s.", currentFlowKey.String()))

		for _, v := range vars {
			result = append(result, v)
		}

		variable := &ResolvedInput{
			FlowKey:  currentFlowKey,
			FullName: fmt.Sprintf("%s.%s", currentFlowKey.String(), input.Name),
			Input:    *input,
		}

		ctx.WithFields(log.Fields{"full": variable.FullName, "flow": variable.FlowKey.String()}).Debugf("has var %s. short=%s", variable.Name, variable.ShortName())

		result = append(result, variable)
	}

	return result
}
