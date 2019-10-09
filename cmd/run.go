package cmd

import (
	variant "github.com/mumoshu/variant/pkg"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"strings"
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

func Run(taskDef *variant.TaskDef, opts variant.Opts) (map[string]string, error) {
	if opts.Log == nil {
		panic("log must be set")
	}
	if opts.CommandPath == "" {
		panic("command path must be set")
	}
	if opts.Args == nil {
		panic("args must be set")
	}

	cobraApp, err := command(opts.CommandPath, taskDef, opts)
	if err != nil {
		return nil, err
	}

	return cobraApp.Run(opts.Args)
}

func command(commandPath string, taskDef *variant.TaskDef, opts variant.Opts) (*variant.CobraApp, error) {
	if opts.Log == nil {
		opts.Log = logrus.StandardLogger()
	}
	if opts.ExtraCmds == nil || len(opts.ExtraCmds) == 0 {
		opts.ExtraCmds = []*cobra.Command{
			EnvCmd,
			VersionCmd(logrus.StandardLogger()),
		}
	}

	return variant.Init(commandPath, taskDef, opts)
}

type Command struct {
	commandPath string
	taskDef     *variant.TaskDef
	opts        variant.Opts
}

func New(commanPath string, taskDef *variant.TaskDef, opts variant.Opts) *Command {
	return &Command{
		commandPath: commanPath,
		taskDef:     taskDef,
		opts:        opts,
	}
}

func (c *Command) Run(args []string) (string, error) {
	cobraApp, err := command(c.commandPath, c.taskDef, c.opts)
	if err != nil {
		return "", err
	}

	results, err := cobraApp.Run(args)
	if err != nil {
		return "", err
	}

	if len(args) == 0 {
		return results[""], nil
	}

	return results[cobraApp.VariantApp.LastRun], nil
}
