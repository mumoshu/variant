package run

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/juju/errors"

	"github.com/mumoshu/variant/cmd"
	"github.com/mumoshu/variant/pkg"
	"github.com/mumoshu/variant/pkg/load"
	"github.com/mumoshu/variant/pkg/util/envutil"
	"github.com/mumoshu/variant/pkg/util/fileutil"
	"github.com/spf13/cobra"
	"path/filepath"
)

func init() {
	logrus.SetOutput(os.Stdout)

	verbose := false
	logtostderr := false
	for _, e := range os.Environ() {
		if strings.Contains(e, "VERBOSE=") {
			verbose = true
			break
		}
		if strings.Contains(e, "LOGTOSTDERR=") {
			logtostderr = true
			break
		}
	}

	if verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if logtostderr {
		logrus.SetOutput(os.Stderr)
	}

	variant.Register(variant.NewTaskStepLoader())
	variant.Register(variant.NewScriptStepLoader())
	variant.Register(variant.NewOrStepLoader())
	variant.Register(variant.NewIfStepLoader())
}

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
			cmd.EnvCmd,
			cmd.BuildCmd,
			cmd.InitCmd,
			cmd.VersionCmd(logrus.StandardLogger()),
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
			cmd.EnvCmd,
			cmd.VersionCmd(logrus.StandardLogger()),
		},
	})
}

func Def(rootTaskConfig *variant.TaskDef, opts variant.Opts) {
	if opts.Log == nil {
		opts.Log = logrus.StandardLogger()
	}
	if opts.CommandPath == "" {
		opts.CommandPath = os.Args[0]
	}
	if opts.Args == nil {
		opts.Args = os.Args[1:]
	}
	if opts.ExtraCmds == nil || len(opts.ExtraCmds) == 0 {
		opts.ExtraCmds = []*cobra.Command{
			cmd.EnvCmd,
			cmd.VersionCmd(logrus.StandardLogger()),
		}
	}

	log := opts.Log
	args := opts.Args

	cobraApp, err := variant.Init(rootTaskConfig, opts)
	if err != nil {
		log.Errorf("%v", err)
		os.Exit(1)
	}

	if err := cobraApp.Run(args); err != nil {
		switch cmdErr := err.(type) {
		case variant.CommandError:
			c := strings.Join(strings.Split(cmdErr.TaskName.String(), "."), " ")
			if log.GetLevel() == logrus.DebugLevel {
				log.Errorf("Stack trace: %+v", err)
			}
			errs := strings.Split(err.Error(), ": ")
			msg := strings.Join(errs, "\n")
			log.Errorf("Error: `%s` failed: %s", c, msg)
			if strings.Trim(cmdErr.Cause, " \n\t") != "" {
				log.Errorf("Caused by: %s", cmdErr.Cause)
			}
		default:
			// Variant command should produce the command help,
			// because it is run without any args and the root command is not defined
			if len(args) == 0 {
				os.Exit(0)
			}
			log.Errorf("Unexpected type of error %T: %s", err, err)
		}
		os.Exit(1)
	}
}
