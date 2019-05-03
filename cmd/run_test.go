package cmd

import (
	"fmt"
	variant "github.com/mumoshu/variant/pkg"
	"github.com/mumoshu/variant/pkg/api/task"

	"testing"
)

func TestVariant(t *testing.T) {
	taskDef := &variant.TaskDef{
		Name:        "Variantfile",
		Description: "",
		Inputs:      variant.InputConfigs{},
		TaskDefs: variant.TaskDefs{
			&variant.TaskDef{
				Name:        "duplicate",
				Description: "",
				Inputs: variant.InputConfigs{
					&variant.InputConfig{
						Name:          "input",
						Description:   "",
						ArgumentIndex: variant.Int(0),
						Type:          "string",
						Default:       "",
						Properties:    map[string]map[string]interface{}(nil),
						Remainings:    map[string]interface{}(nil)},
				},
				TaskDefs: variant.TaskDefs{},
				Script:   "echo \"{{.input}}_{{.input}}\"\n",
				Steps: []variant.Step{
					variant.ScriptStep{
						Name:   "script",
						Code:   "echo \"{{.input}}_{{.input}}\"\n",
						Silent: false,
						RunnerConfig: variant.RunnerConfig{
							Image:      "",
							Command:    "",
							Entrypoint: (*string)(nil),
							Artifacts:  []variant.Artifact(nil),
							Args:       []string(nil),
							Envfile:    "",
							Env:        map[string]string(nil),
							Volumes:    []string(nil),
							Net:        "",
							Workdir:    "",
						},
					},
				},
				Autoenv:     false,
				Autodir:     false,
				Interactive: false,
				Private:     false,
			},
			&variant.TaskDef{
				Name:        "test",
				Description: "",
				Inputs: variant.InputConfigs{
					&variant.InputConfig{
						Name:          "input",
						Description:   "",
						ArgumentIndex: variant.Int(0),
						Type:          "string",
						Default:       nil,
						Properties:    map[string]map[string]interface{}(nil),
						Remainings:    map[string]interface{}(nil),
					},
					&variant.InputConfig{
						Name:          "expected",
						Description:   "",
						ArgumentIndex: variant.Int(1),
						Type:          "string",
						Default:       nil,
						Properties:    map[string]map[string]interface{}(nil),
						Remainings:    map[string]interface{}(nil)}},
				TaskDefs: variant.TaskDefs{},
				Script:   "",
				Steps: []variant.Step{
					variant.TaskStep{
						Name:          "res",
						TaskKeyString: "duplicate",
						Arguments:     task.Arguments{"input": "{{.input}}"},
						Silent:        false,
					},
					variant.ScriptStep{
						Name:   "actual",
						Code:   "echo \"Double {{.input}} will be: {{.res}}\"\n",
						Silent: false,
						RunnerConfig: variant.RunnerConfig{
							Image: "", Command: "",
							Entrypoint: (*string)(nil),
							Artifacts:  []variant.Artifact(nil),
							Args:       []string(nil),
							Envfile:    "",
							Env:        map[string]string(nil),
							Volumes:    []string(nil), Net: "", Workdir: "",
						},
					},
					variant.ScriptStep{
						Name:   "test",
						Code:   "echo \"{{ .actual }}\" | grep \"{{ .expected }}\"\n",
						Silent: false,
						RunnerConfig: variant.RunnerConfig{
							Image:      "",
							Command:    "",
							Entrypoint: (*string)(nil),
							Artifacts:  []variant.Artifact(nil),
							Args:       []string(nil),
							Envfile:    "",
							Env:        map[string]string(nil),
							Volumes:    []string(nil),
							Net:        "",
							Workdir:    "",
						},
					},
				},
				Autoenv:     false,
				Autodir:     false,
				Interactive: false,
				Private:     false,
			},
		},
		Script: "",
		Steps: []variant.Step{
			variant.TaskStep{
				Name:          "step-1",
				TaskKeyString: "test",
				Arguments: task.Arguments{
					"expected": "Double FOO will be: FOO_FOO",
					"input":    "FOO",
				},
				Silent: false,
			},
		},
		Autoenv:     false,
		Autodir:     false,
		Interactive: false,
		Private:     false,
	}

	adhocTaskInputs := variant.InputConfigs{&variant.InputConfig{
		Name:          "input",
		Description:   "",
		ArgumentIndex: variant.Int(0),
		Type:          "string",
		Default:       nil,
		Properties:    map[string]map[string]interface{}(nil),
		Remainings:    map[string]interface{}(nil),
	}}
	adhocTaskDef := &variant.TaskDef{Inputs: adhocTaskInputs}
	if err := taskDef.Add([]string{"should"}, &variant.TaskDef{Name: "should"}, func(_ variant.ExecutionContext) (string, error) {
		return "", nil
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := taskDef.Add([]string{"should", "succeed"}, adhocTaskDef, func(ctx variant.ExecutionContext) (string, error) {
		params := ctx.Values()
		in := params["input"]
		return fmt.Sprintf("%s_%s_%s", in, in, in), nil
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := taskDef.Add([]string{"should", "fail"}, adhocTaskDef, func(ctx variant.ExecutionContext) (string, error) {
		return "", fmt.Errorf("simulated error")
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	//bytes, err := json.MarshalIndent(taskDef, "", "  ")
	//panic(fmt.Sprintf("%s", string(bytes)))

	cmd := New("variant", taskDef, variant.Opts{})

	out1, err := cmd.Run([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("outs1: %s", out1)
	expected1 := `FOO_FOO
Double FOO will be: FOO_FOO
Double FOO will be: FOO_FOO`
	if out1 != expected1 {
		t.Fatalf("unexpected out1: %s", out1)
	}

	out2, err := cmd.Run([]string{"should", "succeed", "--input=SUCCEED"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("out2: %s", out2)
	if out2 != "SUCCEED_SUCCEED_SUCCEED" {
		t.Fatalf("unexpected out2: %s", out2)
	}

	out3, err := cmd.Run([]string{"should", "fail", "--input=FAIL"})
	if err == nil {
		t.Fatal("expected error, but succeeded")
	}

	if out3 != "" {
		t.Fatalf("unexpected out3: %s", out3)
	}
}
