package envutil

import (
	"os"
	"strings"
)

func ParseEnviron() map[string]string {
	mergedEnv := map[string]string{}

	for _, pair := range os.Environ() {
		splits := strings.SplitN(pair, "=", 2)
		key, value := splits[0], splits[1]
		mergedEnv[key] = value
	}

	return mergedEnv
}
