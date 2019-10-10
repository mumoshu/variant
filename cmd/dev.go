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
		HandleErrorAndExit(err, opts)
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
		HandleErrorAndExit(err, opts)
	}
}

func HandleErrorAndExit(err error, opts variant.Opts) {
	msg, status := HandleError(err, opts)
	LogAndExit(opts, msg, status)
}

func LogAndExit(opts variant.Opts, msg string, status int) {
	if msg != "" {
		opts.Log.Errorf("%s", msg)
	}
	os.Exit(status)
}

func HandleError(err error, opts variant.Opts) (string, int) {
	if err == nil {
		return "", 0
	}
	args := opts.Args
	log := opts.Log
	var msg string
	switch cmdErr := err.(type) {
	case variant.InitError:
		msg = fmt.Sprintf("%v", err)
	case variant.CommandError:
		if log.GetLevel() == logrus.DebugLevel {
			msg = fmt.Sprintf("Stack trace: %+v\n", err)
		}
		errs := strings.Split(err.Error(), ": ")
		msg += strings.Join(errs, "\n")
		msg += fmt.Sprintf("\nError: %s", msg)
		if strings.Trim(cmdErr.Cause, " \n\t") != "" {
			msg += fmt.Sprintf("\nCaused by: %s", cmdErr.Cause)
		}
	case variant.InternalError:
		msg = fmt.Sprintf("%v", err)
	default:
		// Variant command should produce the command help,
		// because it is run without any args and the root command is not defined
		if len(args) == 0 {
			return "", 0
		}
		msg = fmt.Sprintf("Unexpected type of error %T: %s", err, err)
	}
	return msg, 1
}

func GetStatus(err error, opts variant.Opts) int {
	switch err.(type) {
	case variant.InitError:
		return 1
	case variant.CommandError:
		return 1
	default:
		// Variant command should produce the command help,
		// because it is run without any args and the root command is not defined
		if len(opts.Args) == 0 {
			return 0
		}
	}
	return 1
}
