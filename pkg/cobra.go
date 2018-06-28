package variant

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/juju/errors"
	flowApi "github.com/mumoshu/variant/pkg/api/task"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"os"
	"strings"
)

type CobraAdapter struct {
	app *Application
}

func NewCobraAdapter(app *Application) *CobraAdapter {
	return &CobraAdapter{
		app: app,
	}
}

func (p *CobraAdapter) GenerateCommand(task *Task, rootCommand *cobra.Command) (*cobra.Command, error) {
	positionalArgs := ""
	for i, input := range task.Inputs {
		if i != len(task.Inputs)-1 {
			positionalArgs += fmt.Sprintf("[%s ", input.Name)
		} else {
			positionalArgs += fmt.Sprintf("[%s", input.Name)
		}
	}
	for i := 0; i < len(task.Inputs); i++ {
		positionalArgs += "]"
	}

	var cmd = &cobra.Command{
		Use: fmt.Sprintf("%s %s", task.Name.Simple(), positionalArgs),
	}
	if task.Description != "" {
		cmd.Short = task.Description
		cmd.Long = task.Description
	}

	taskName := task.Name

	if len(task.Steps) > 0 {
		cmd.Run = func(cmd *cobra.Command, args []string) {
			p.app.UpdateLoggingConfiguration()

			output, err := p.app.RunTaskForKey(taskName, args, flowApi.NewProvidedInputs())

			if err != nil {
				c := strings.Join(strings.Split(taskName.String(), "."), " ")
				stack := strings.Split(errors.ErrorStack(err), "\n")
				for i := len(stack)/2 - 1; i >= 0; i-- {
					opp := len(stack) - 1 - i
					stack[i], stack[opp] = stack[opp], stack[i]
				}
				log.WithFields(log.Fields{"stack": errors.ErrorStack(err)}).Errorf("command %s failed: %v", c, err)
				//log.Errorf("Command `%s` failed\n\nCaused by:\n%s", c, strings.Join(stack, "\n"))
				//log.Debugf("Stack:\n%v", errors.ErrorStack(errors.Trace(err)))
				os.Exit(1)
			}

			fmt.Println(output)
		}
	}

	if rootCommand != nil {
		rootCommand.AddCommand(cmd)
	}

	log.WithFields(log.Fields{"prefix": taskName.String()}).Debug("is a task")

	for _, f := range task.Tasks {
		p.GenerateCommand(f, cmd)
	}

	task.Command = cmd

	return cmd, nil
}

func (p *CobraAdapter) Tasks() map[string]*Task {
	return p.app.Tasks()
}

func (p *CobraAdapter) GenerateAllFlags() {
	for _, task := range p.Tasks() {
		for _, input := range task.ResolvedInputs {
			log.Debugf("Configuring flag and config key for task %s's input: %s", task.Name.String(), input.Name)

			flowConfig := task.TaskConfig
			cmd := task.Command
			var description string
			if input.Description != "" {
				description = input.Description
			} else {
				description = input.Name
			}

			var name string
			if input.TaskKey.String() == task.Name.String() {
				name = input.Name
			} else {
				name = input.ShortName()
			}

			log.Debugf("short=%s, full=%s, name=%s, selected=%s", input.ShortName(), input.FullName, input.Name, name)

			flagName := strings.Replace(name, ".", "-", -1)

			var keyForConfigFromFlag string
			if input.TaskKey.ShortString() == task.Name.ShortString() {
				keyForConfigFromFlag = input.ShortName()
			} else {
				keyForConfigFromFlag = fmt.Sprintf("%s.%s", task.Name.ShortString(), input.ShortName())
			}
			keyForConfigFromFlag = fmt.Sprintf("flags.%s", keyForConfigFromFlag)

			var flagset *pflag.FlagSet
			if len(flowConfig.TaskConfigs) == 0 {
				flagset = cmd.Flags()
				log.Debugf("Binding flag --%s of the command %s to the input %s with the config key %s", flagName, task.Name.ShortString(), input.Name, keyForConfigFromFlag)
			} else {
				flagset = cmd.PersistentFlags()
				log.Debugf("Binding persistent flag --%s to the config key %s", flagName, keyForConfigFromFlag)
			}

			flagset.StringP(flagName, "" /*string(input.Name[0])*/, "", description)

			viper.BindPFlag(keyForConfigFromFlag, flagset.Lookup(flagName))
			//
			//if input.Required() {
			//	if len(flowConfig.TaskConfigs) == 0 {
			//		cmd.MarkFlagRequired(flagName)
			//	} else {
			//		cmd.MarkPersistentFlagRequired(flagName)
			//	}
			//}
		}
	}
}
