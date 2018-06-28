package engine

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
)

type InputResolver interface {
	ResolveInputs()
	ResolveInputsForTask(flowDef *Task) []*ResolvedInput
	ResolveInputsForTaskKey(currentTaskKey TaskName, path string) []*ResolvedInput
}

type RegistryBasedInputResolver struct {
	InputResolver
	registry       *TaskRegistry
	flowKeyCreator *TaskNamer
}

func NewRegistryBasedInputResolver(registry *TaskRegistry, flowKeyCreator *TaskNamer) InputResolver {
	return &RegistryBasedInputResolver{
		registry:       registry,
		flowKeyCreator: flowKeyCreator,
	}
}

func (r *RegistryBasedInputResolver) ResolveInputs() {
	for _, flow := range r.registry.Tasks() {
		flow.ResolvedInputs = r.ResolveInputsForTask(flow)
	}
}

func (r *RegistryBasedInputResolver) ResolveInputsForTask(flowDef *Task) []*ResolvedInput {
	return r.ResolveInputsForTaskKey(flowDef.Name, "")
}

func (r *RegistryBasedInputResolver) ResolveInputsForTaskKey(currentTaskKey TaskName, path string) []*ResolvedInput {
	result := []*ResolvedInput{}

	ctx := log.WithFields(log.Fields{"prefix": fmt.Sprintf("%s", currentTaskKey.String())})

	currentTask, err := r.registry.FindTask(currentTaskKey)

	if err != nil {
		allTasks := r.registry.AllTaskKeys()
		ctx.Debugf("is not a Task in: %v", allTasks)
		return []*ResolvedInput{}
	}

	for _, input := range currentTask.Inputs {
		childKey := r.flowKeyCreator.FromInput(input)

		ctx.Debugf("depends on %s", childKey.String())

		vars := r.ResolveInputsForTaskKey(childKey, fmt.Sprintf("%s.", currentTaskKey.String()))

		for _, v := range vars {
			result = append(result, v)
		}

		variable := &ResolvedInput{
			TaskKey:  currentTaskKey,
			FullName: fmt.Sprintf("%s.%s", currentTaskKey.String(), input.Name),
			Input:    *input,
		}

		ctx.WithFields(log.Fields{"full": variable.FullName, "task": variable.TaskKey.String()}).Debugf("has var %s. short=%s", variable.Name, variable.ShortName())

		result = append(result, variable)
	}

	return result
}
