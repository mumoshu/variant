package variant

import (
	"github.com/mattn/go-shellwords"
	"os"
	"strings"
)

func ArgsFromEnvVars() ([]string, error) {
	return argsFromEnvVars(os.Getenv)
}

func argsFromEnvVars(getenv func(string) string) ([]string, error) {
	const (
		Run           = "VARIANT_RUN"
		RunTrimPrefix = "VARIANT_RUN_TRIM_PREFIX"
	)

	run := getenv(Run)
	prefix := getenv(RunTrimPrefix)

	if run != "" {
		run = strings.TrimSpace(run)
		if prefix != "" {
			run = strings.TrimPrefix(run, prefix)
		}

		return shellwords.Parse(run)
	}
	return nil, nil
}
