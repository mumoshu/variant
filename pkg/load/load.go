package load

import (
	"github.com/mumoshu/variant/pkg"
	"io/ioutil"
	"path/filepath"
)

func File(cmdPath string) (*variant.TaskDef, error) {
	cmdName := filepath.Base(cmdPath)

	yaml, err := ioutil.ReadFile(cmdPath)
	if err != nil {
		return nil, err
	}

	taskDef, err := YAML(string(yaml))
	if err != nil {
		return nil, err
	}

	taskDef.Name = cmdName

	return taskDef, nil
}

func YAML(yaml string) (*variant.TaskDef, error) {
	return variant.ReadTaskDefFromBytes([]byte(yaml))
}
