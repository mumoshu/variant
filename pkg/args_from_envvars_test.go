package variant

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"testing"
)

func TestArgsFromEnvVars(t *testing.T) {
	testcases := []struct {
		run        string
		trimPrefix string
		expected   []string
	}{
		{
			run:        "/foo bar --a=b",
			trimPrefix: "",
			expected:   []string{"/foo", "bar", "--a=b"},
		},
		{
			run:        "/foo bar --a=b ",
			trimPrefix: "",
			expected:   []string{"/foo", "bar", "--a=b"},
		},
		{
			run:        " /foo bar --a=b ",
			trimPrefix: "",
			expected:   []string{"/foo", "bar", "--a=b"},
		},
		{
			run:        "/foo bar --a=b",
			trimPrefix: "/foo",
			expected:   []string{"bar", "--a=b"},
		},
		{
			run:        "/foo bar --a=b ",
			trimPrefix: "/foo",
			expected:   []string{"bar", "--a=b"},
		},
		{
			run:        " /foo bar --a=b",
			trimPrefix: "/foo",
			expected:   []string{"bar", "--a=b"},
		},
	}

	for i, tc := range testcases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			getenv := func(name string) string {
				switch name {
				case "VARIANT_RUN":
					return tc.run
				case "VARIANT_RUN_TRIM_PREFIX":
					return tc.trimPrefix
				default:
					t.Fatalf("Unexpected envvar accessed: %s", name)
					return ""
				}
			}
			args, err := argsFromEnvVars(getenv)
			if diff := cmp.Diff(tc.expected, args); diff != "" {
				t.Errorf("%v", diff)
			}

			if err != nil {
				t.Errorf("%v", err)
			}
		})
	}
}
