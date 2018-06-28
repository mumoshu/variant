package variant

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/juju/errors"
	flowApi "github.com/mumoshu/variant/pkg/api/task"
	"github.com/spf13/cobra"
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
				log.WithFields(log.Fields{"stack": errors.ErrorStack(err)}).Errorf("command %s failed", c)
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
	for _, flow := range p.Tasks() {
		for _, input := range flow.ResolvedInputs {
			log.Debugf("Configuring flag and config key for task %s's input: %s", flow.Name.String(), input.Name)

			flowConfig := flow.TaskConfig
			cmd := flow.Command
			var description string
			if input.Description != "" {
				description = input.Description
			} else {
				description = input.Name
			}

			var name string
			if input.TaskKey.String() == flow.Name.String() {
				name = input.Name
			} else {
				name = input.ShortName()
			}

			log.Debugf("short=%s, full=%s, name=%s, selected=%s", input.ShortName(), input.FullName, input.Name, name)

			flagName := strings.Replace(name, ".", "-", -1)

			var keyForConfigFromFlag string
			if input.TaskKey.ShortString() == flow.Name.ShortString() {
				keyForConfigFromFlag = input.ShortName()
			} else {
				keyForConfigFromFlag = fmt.Sprintf("%s.%s", flow.Name.ShortString(), input.ShortName())
			}
			keyForConfigFromFlag = fmt.Sprintf("flags.%s", keyForConfigFromFlag)

			if len(flowConfig.TaskConfigs) == 0 {
				cmd.Flags().StringP(flagName, "" /*string(input.Name[0])*/, "", description)
				//log.Debugf("Binding flag --%s to the config key %s", flagName, name)
				//viper.BindPFlag(name, cmd.Flags().Lookup(flagName))
				log.Debugf("Binding flag --%s of the command %s to the input %s with the config key %s", flagName, flow.Name.ShortString(), input.Name, keyForConfigFromFlag)
				viper.BindPFlag(keyForConfigFromFlag, cmd.Flags().Lookup(flagName))
			} else {
				cmd.PersistentFlags().StringP(flagName, "" /*string(input.Name[0])*/, "" /*default*/, description)
				//log.Debugf("Binding persistent flag --%s to the config key %s", flagName, name)
				//viper.BindPFlag(name, cmd.PersistentFlags().Lookup(flagName))
				log.Debugf("Binding persistent flag --%s to the config key %s", flagName, keyForConfigFromFlag)
				viper.BindPFlag(keyForConfigFromFlag, cmd.PersistentFlags().Lookup(flagName))
			}
		}
	}
}
