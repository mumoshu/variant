package variant

import (
	"strings"

	log "github.com/sirupsen/logrus"

	"fmt"

	"github.com/pkg/errors"
)

type TaskRunner struct {
	*Task
	Values   map[string]interface{}
	Template *TaskTemplate
}

type stepCaller struct {
	task *Task
}

func (c stepCaller) GetKey() Key {
	return c.task.Name.AsStepKey()
}

func (t TaskRunner) AsStepCaller() Caller {
	return stepCaller{
		task: t.Task,
	}
}

func NewTaskRunner(taskDef *Task, taskTemplate *TaskTemplate, vars map[string]interface{}) (TaskRunner, error) {
	runner := TaskRunner{
		Values:   vars,
		Task:     taskDef,
		Template: taskTemplate,
	}
	return runner, nil
}

func (t TaskRunner) GetKey() Key {
	return t.Name.AsStepKey()
}

func (t TaskRunner) GenerateAutoenv() (map[string]string, error) {
	replacer := strings.NewReplacer("-", "_", ".", "_")
	toEnvName := func(parName string) string {
		return strings.ToUpper(replacer.Replace(parName))
	}
	return t.GenerateAutoenvRecursively("", t.Values, toEnvName)
}

func (t TaskRunner) GenerateAutoenvRecursively(path string, env map[string]interface{}, toEnvName func(string) string) (map[string]string, error) {
	logger := log.WithFields(log.Fields{"path": path})
	result := map[string]string{}
	for k, v := range env {
		if nestedEnv, ok := v.(map[string]interface{}); ok {
			nestedResult, err := t.GenerateAutoenvRecursively(fmt.Sprintf("%s.", k), nestedEnv, toEnvName)
			if err != nil {
				logger.Errorf("Error while recursiong: %v", err)
			}
			for k, v := range nestedResult {
				result[k] = v
			}
		} else if nestedEnv, ok := v.(map[string]string); ok {
			for k2, v := range nestedEnv {
				result[toEnvName(fmt.Sprintf("%s%s.%s", path, k, k2))] = v
			}
		} else if ary, ok := v.([]string); ok {
			for i, v := range ary {
				result[toEnvName(fmt.Sprintf("%s%s.%d", path, k, i))] = v
			}
		} else {
			if stringV, ok := v.(string); ok {
				result[toEnvName(fmt.Sprintf("%s%s", path, k))] = stringV
			} else if v == nil {
				result[toEnvName(fmt.Sprintf("%s%s", path, k))] = ""
			} else if fmt.Sprintf("%T", v) == "bool" {
				result[toEnvName(fmt.Sprintf("%s%s", path, k))] = fmt.Sprintf("%t", v)
			} else if fmt.Sprintf("%T", v) == "int" {
				result[toEnvName(fmt.Sprintf("%s%s", path, k))] = fmt.Sprintf("%d", v)
			} else {
				return nil, errors.Errorf("The value for the key %s was neither a `map[string]interface{}` nor a `string`: %v(%#v)", k, v, v)
			}
		}
	}
	logger.Debugf("Generated autoenv: %v", result)
	return result, nil
}

func (t *TaskRunner) Run(project *Application, asInput bool, caller ...*Task) (string, error) {
	var ctx *log.Entry

	if len(caller) > 0 {
		ctx = log.WithFields(log.Fields{"caller": caller[0].GetKey().ShortString()})
	} else {
		ctx = log.WithFields(log.Fields{})
	}

	ctx.Debugf("task %s started", t.Name.String())

	var output StepStringOutput
	var lastout StepStringOutput
	var err error

	context := NewStepExecutionContext(*project, *t, t.Template, asInput, append(append([]*Task{t.Task}, caller...)))

	if t.TaskDef.fun != nil {
		return t.TaskDef.fun(context)
	}

	for _, s := range t.Steps {
		lastout, err = s.Run(context)

		if err != nil {
			return lastout.String, errors.Wrap(err, "Task#Run failed while running a script")
		}

		if s.GetName() != "" {
			context = context.WithAdditionalValues(map[string]interface{}{s.GetName(): lastout.String})
		}

		if !s.Silenced() && len(lastout.String) > 0 {
			var sep string
			if output.String != "" && !strings.HasSuffix(output.String, "\n") {
				sep = "\n"
			}
			output = StepStringOutput{
				output.String + sep + lastout.String,
			}
		}
	}
	if output.String == "" {
		output = lastout
	}

	if err != nil {
		err = errors.Wrap(err, "Task#Run failed while running a script")
	}

	ctx.Debugf("task %s finished. out=%v, err=%v", t.Name.String(), output, err)

	return output.String, err
}
