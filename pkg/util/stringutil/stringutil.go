package stringutil

import (
	"github.com/huandu/xstrings"
	"regexp"
	"strings"
)

var (
	regex            = regexp.MustCompile(`-([0-9]+)`)
	argumentReplacer = strings.NewReplacer(".", "-")
	envReplacer      = strings.NewReplacer("-", "_", ".", "_")
)

func ToArgumentName(name string) string {
	n := strings.Trim(regex.ReplaceAllString(xstrings.ToKebabCase(name), "$1-"), "-")
	return argumentReplacer.Replace(n)
}

func ToEnvironmentName(name string) string {
	n := strings.Trim(regex.ReplaceAllString(xstrings.ToKebabCase(name), "$1-"), "-")
	return strings.ToUpper(envReplacer.Replace(n))
}
