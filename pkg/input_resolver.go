package variant

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
)

type InputResolver interface {
	ResolveInputs()
	ResolveInputsForTask(flowDef *Task) []*Input
	ResolveInputsForTaskKey(currentTaskKey TaskName, path string) []*Input
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

func (r *RegistryBasedInputResolver) ResolveInputsForTask(flowDef *Task) []*Input {
	return r.ResolveInputsForTaskKey(flowDef.Name, "")
}

func (r *RegistryBasedInputResolver) ResolveInputsForTaskKey(currentTaskKey TaskName, path string) []*Input {
	inputs := []*Input{}

	ctx := log.WithFields(log.Fields{"prefix": fmt.Sprintf("%s", currentTaskKey.String())})

	currentTask := r.registry.FindTask(currentTaskKey)

	if currentTask == nil {
		allTasks := r.registry.AllTaskKeys()
		ctx.Debugf("is not a Task in: %v", allTasks)
		return []*Input{}
	}

	for _, input := range currentTask.Inputs {
		childKey := r.flowKeyCreator.FromInput(input)

		ctx.Debugf("depends on %s", childKey.String())

		vars := r.ResolveInputsForTaskKey(childKey, fmt.Sprintf("%s.", currentTaskKey.String()))

		for _, v := range vars {
			inputs = append(inputs, v)
		}

		input := &Input{
			TaskKey:     currentTaskKey,
			FullName:    fmt.Sprintf("%s.%s", currentTaskKey.String(), input.Name),
			InputConfig: *input,
		}

		ctx.WithFields(log.Fields{"full": input.FullName, "task": input.TaskKey.String()}).Debugf("has var %s. short=%s", input.Name, input.ShortName())

		inputs = append(inputs, input)
	}

	return inputs
}
