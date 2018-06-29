package step

import (
	"github.com/mumoshu/variant/pkg/api/task"
	"text/template"
)

type ExecutionContext interface {
	GenerateAutoenv() (map[string]string, error)
	Caller() []Caller
	Key() Key
	Vars() map[string]interface{}
	CreateFuncMap() template.FuncMap
	ProjectName() string
	Autoenv() bool
	Autodir() bool
	Interactive() bool
	RunAnotherTask(key string, provided task.Arguments) (string, error)
}
