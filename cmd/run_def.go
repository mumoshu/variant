package cmd

import (
	variant "github.com/mumoshu/variant/pkg"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

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
	log := opts.Log
	args := opts.Args

	if _, err := Run(opts.CommandPath, args, rootTaskConfig, opts); err != nil {
		switch cmdErr := err.(type) {
		case variant.InitError:
			log.Errorf("%v", err)
			os.Exit(1)
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
