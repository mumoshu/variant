package sandbox

import (
	"gopkg.in/yaml.v2"
	"reflect"
	"strings"
	"testing"

	"fmt"
	"github.com/davecgh/go-spew/spew"

	"github.com/kr/pretty"
	"os"
)

type TestHelper struct {
	t *testing.T
}

func (h TestHelper) AssertFlowFindsAnotherFlow(flow ScopedFlow, path string, expected ScopedFlow) {
	expr, err := flow.Scope().FindFlowAtPath(path)

	if err != nil {
		h.t.Errorf("%s", err)
	}

	actual := *expr

	if !reflect.DeepEqual(actual, expected) {
		h.t.Errorf("%s->%s doesn't match expected value:\nactual=%s\nexpected=%s\ndiff=%s", flow.Path(), path, spew.Sdump(actual), spew.Sdump(expected), strings.Join(pretty.Diff(actual, expected), "\n"))
	}
}

func (h TestHelper) AssertFlowFindsNothing(flow ScopedFlow, path string) {
	expr, err := flow.Scope().FindFlowAtPath(path)

	if err == nil {
		h.t.Errorf("%s.Find(path=\"%s\") is expected to fail but succeeded: %v", flow, path, expr)
	}
}

func (h TestHelper) AssertEquals(actual interface{}, expected interface{}) {
	if !reflect.DeepEqual(actual, expected) {
		h.t.Errorf("actual value %s doesn't match expected value %s\ndiff=%s", spew.Sdump(actual), spew.Sdump(expected), strings.Join(pretty.Diff(actual, expected), "\n"))
	}
}

func TestPath(t *testing.T) {
	a := &Flow{
		Name: "a",
	}

	b := &Flow{
		Name: "b",
	}

	expr := ScopedFlow{
		Stack: NewStackFromFlows(a, b),
	}

	h := TestHelper{t: t}
	h.AssertEquals(expr.Path(), "a.b")
}

func TestMinimalConfigParsing(t *testing.T) {
	const data = `
name: root
script: root
flows:
- name: a
  flows:
  - name: a
    script: echo .a.a
    flows:
    - name: a
      script: echo .a.a.a
  - name: b
    inputs: [ {"name": c} ]
    script: echo .a.b
  - name: c
    script: echo .a.c
- name: c
  script: echo .c
env:
  FOO: foo
`
	flowFromYaml := Flow{}

	if err := yaml.Unmarshal([]byte(data), &flowFromYaml); err != nil {
		t.Errorf("error: %v", err)
	}

	flow_a_a_a := Flow{
		Name:   "a",
		Script: "echo .a.a.a",
	}

	flow_a_a := Flow{
		Name:   "a",
		Script: "echo .a.a",
		Flows: []Flow{
			flow_a_a_a,
		},
	}

	flow_a_b := Flow{
		Name: "b",
		Inputs: []Input{
			Input{
				Name: "c",
			},
		},
		Script: "echo .a.b",
	}

	flow_a_c := Flow{
		Name:   "c",
		Script: "echo .a.c",
	}

	flow_a := Flow{
		Name: "a",
		Flows: []Flow{
			flow_a_a,
			flow_a_b,
			flow_a_c,
		},
	}

	flow_c := Flow{
		Name:   "c",
		Script: "echo .c",
	}

	flow_root := Flow{
		Name: "root",
		Flows: []Flow{
			flow_a,
			flow_c,
		},
		Script: "root",
		Env:    map[string]string{"FOO": "foo"},
	}

	if !reflect.DeepEqual(flowFromYaml, flow_root) {
		t.Errorf("actual value %s doesn't match expected value %s", spew.Sdump(flowFromYaml), spew.Sdump(flow_root))
	}

	ctx_root := flow_root.AsRoot()

	findFlowInContext := func(path string) ScopedFlow {
		found, err := ctx_root.Scope().FindFlowAtPath(path)

		if err != nil {
			t.Errorf("%s", err)
		}

		return *found
	}

	ctx_a := findFlowInContext("a")
	ctx_a_a := findFlowInContext("a.a")
	ctx_a_a_a := findFlowInContext("a.a.a")
	ctx_a_b := findFlowInContext("a.b")
	ctx_a_c := findFlowInContext("a.c")
	ctx_c := findFlowInContext("c")

	h := TestHelper{t: t}

	h.AssertFlowFindsAnotherFlow(*ctx_root, "a", ctx_a)
	h.AssertFlowFindsAnotherFlow(*ctx_root, "c", ctx_c)

	h.AssertFlowFindsAnotherFlow(ctx_a, "a", ctx_a_a)
	h.AssertFlowFindsAnotherFlow(ctx_a, "b", ctx_a_b)
	h.AssertFlowFindsAnotherFlow(ctx_a, "c", ctx_a_c)

	h.AssertEquals(ctx_c.GetName(), "c")
	//h.AssertEquals(ctx_c.Stack[1].Name, "c")
	//h.AssertEquals(ctx_c.Stack[0].Name, "root")
	h.AssertEquals(ctx_c.Path(), "root.c")

	os.Stderr.WriteString(fmt.Sprintf("%s\n", ctx_c))

	h.AssertFlowFindsAnotherFlow(ctx_c, "a", ctx_a)
	h.AssertFlowFindsNothing(ctx_c, "b")
	h.AssertFlowFindsAnotherFlow(ctx_c, "c", ctx_c)

	// ".a", ".a.a", ".a.a.a" are in scope of ".a.a". A child is preffered to a sibling
	h.AssertFlowFindsAnotherFlow(ctx_a_a, "a", ctx_a_a_a)
	h.AssertFlowFindsAnotherFlow(ctx_a_a, "b", ctx_a_b)
	h.AssertFlowFindsAnotherFlow(ctx_a_a, "c", ctx_a_c)

	// ".a", ".a.a" are in scope of ".a.b". A sibling is preferred to a parent
	h.AssertFlowFindsAnotherFlow(ctx_a_b, "a", ctx_a_a)
	h.AssertFlowFindsAnotherFlow(ctx_a_b, "b", ctx_a_b)
	h.AssertFlowFindsAnotherFlow(ctx_a_b, "c", ctx_a_c)
	// ".a", ".a.a" are in scope of ".a.c". A sibling is preferred to a parent
	h.AssertFlowFindsAnotherFlow(ctx_a_c, "a", ctx_a_a)
	// The sibling ".a.b" is in scope of ".a.c"
	h.AssertFlowFindsAnotherFlow(ctx_a_c, "b", ctx_a_b)
	// The flow ".a.c" is in its own scope
	h.AssertFlowFindsAnotherFlow(ctx_a_c, "c", ctx_a_c)
	// ".a", "a.a", and ".a.a.a" is in scope ".a.a.a". Itself is preferred.
	h.AssertFlowFindsAnotherFlow(ctx_a_a_a, "a", ctx_a_a_a)

	//
	//if !reflect.DeepEqual(flow.Prepare("a.b"), Scope {
	//	Stack: [root, root_a, root_b],
	//})
}
