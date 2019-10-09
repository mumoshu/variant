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

func MustRun() {
	if opts, err := RunE(); err != nil {
		HandleError(err, opts)
	}
}

func RunE() (variant.Opts, error) {
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

	opts := variant.Opts{
		CommandPath: cmdPath,
		Args:        args,
		Log:         logrus.StandardLogger(),
	}

	additionalArgs, err := variant.ArgsFromEnvVars()
	if err != nil {
		return opts, variant.NewInitError(err)
	}
	args = append(args, additionalArgs...)

	opts.Args = args

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
			return opts, variant.NewInitError(err)
		}
		taskDef = taskConfigFromFile
	} else {
		taskDef = variant.NewDefaultTaskConfig()
	}

	taskDef.Name = cmdName

	opts.ExtraCmds = []*cobra.Command{
		EnvCmd,
		BuildCmd,
		InitCmd,
		UtilsCmd,
		VersionCmd(logrus.StandardLogger()),
	}

	_, err = Run(taskDef, opts)
	return opts, err
}

func YAML(yaml string) {
	cmdPath := os.Args[0]
	taskDef, err := load.YAML(yaml)

	if err != nil {
		logrus.Errorf("%+v", err)
		panic(errors.Trace(err))
	}

	taskDef.Name = filepath.Base(cmdPath)

	opts := variant.Opts{
		CommandPath: cmdPath,
		Args:        os.Args[1:],
		Log:         logrus.StandardLogger(),
		ExtraCmds: []*cobra.Command{
			EnvCmd,
			VersionCmd(logrus.StandardLogger()),
		},
	}

	if _, err := Run(taskDef, opts); err != nil {
		HandleError(err, opts)
	}
}

func HandleError(err error, opts variant.Opts) {
	args := opts.Args
	log := opts.Log
	switch cmdErr := err.(type) {
	case variant.InitError:
		log.Errorf("%v", err)
		os.Exit(1)
	case variant.CommandError:
		if log.GetLevel() == logrus.DebugLevel {
			log.Errorf("Stack trace: %+v", err)
		}
		errs := strings.Split(err.Error(), ": ")
		msg := strings.Join(errs, "\n")
		log.Errorf("Error: %s", msg)
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
