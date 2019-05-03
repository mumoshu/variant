package cmd

import (
	"fmt"
	"github.com/juju/errors"
	variant "github.com/mumoshu/variant/pkg"
	"github.com/mumoshu/variant/pkg/load"
	"github.com/mumoshu/variant/pkg/util/envutil"
	"github.com/mumoshu/variant/pkg/util/fileutil"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"strings"
)

func Dev() {
	var taskDef *variant.TaskDef
	var args []string

	var cmdName string
	var cmdPath string
	var varfile string

	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") && fileutil.Exists(os.Args[1]) {
		varfile = os.Args[1]
		args = os.Args[2:]
		cmdPath = varfile
		cmdName = filepath.Base(cmdPath)
	} else {
		cmdPath = os.Args[0]
		cmdName = filepath.Base(cmdPath)
		varfile = fmt.Sprintf("%s.definition.yaml", cmdName)
		args = os.Args[1:]
	}

	environ := envutil.ParseEnviron()

	if environ["VARFILE"] != "" {
		varfile = environ["VARFILE"]
	}

	if !fileutil.Exists(varfile) {
		varfile = "Variantfile"
	}

	if fileutil.Exists(varfile) {
		taskConfigFromFile, err := variant.ReadTaskDefFromFile(varfile)

		if err != nil {
			logrus.Errorf("%+v", err)
			panic(errors.Trace(err))
		}
		taskDef = taskConfigFromFile
	} else {
		taskDef = variant.NewDefaultTaskConfig()
	}

	taskDef.Name = cmdName

	Def(taskDef, variant.Opts{
		CommandPath: cmdPath,
		Args:        args,
		Log:         logrus.StandardLogger(),
		ExtraCmds: []*cobra.Command{
			EnvCmd,
			BuildCmd,
			InitCmd,
			VersionCmd(logrus.StandardLogger()),
		},
	})
}

func YAML(yaml string) {
	cmdPath := os.Args[0]
	taskDef, err := load.YAML(yaml)

	if err != nil {
		logrus.Errorf("%+v", err)
		panic(errors.Trace(err))
	}

	taskDef.Name = filepath.Base(cmdPath)

	Def(taskDef, variant.Opts{
		CommandPath: cmdPath,
		Args:        os.Args[1:],
		Log:         logrus.StandardLogger(),
		ExtraCmds: []*cobra.Command{
			EnvCmd,
			VersionCmd(logrus.StandardLogger()),
		},
	})
}
