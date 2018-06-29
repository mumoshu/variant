package step

import (
	"github.com/mumoshu/variant/pkg/api/task"
)

type ExecutionContext interface {
	GenerateAutoenv() (map[string]string, error)
	Caller() []Caller
	Key() Key
	Vars() map[string]interface{}
	Autoenv() bool
	Autodir() bool
	Interactive() bool
	Render(expr string, name string) (string, error)
	RunAnotherTask(key string, arguments task.Arguments, scope map[string]interface{}) (string, error)
}
