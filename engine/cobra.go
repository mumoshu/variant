package engine

import (
	flowApi "../api/flow"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/juju/errors"
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

func (p *CobraAdapter) GenerateCommand(flow *Flow, rootCommand *cobra.Command) (*cobra.Command, error) {
	positionalArgs := ""
	for i, input := range flow.Inputs {
		if i != len(flow.Inputs)-1 {
			positionalArgs += fmt.Sprintf("[%s ", input.Name)
		} else {
			positionalArgs += fmt.Sprintf("[%s", input.Name)
		}
	}
	for i := 0; i < len(flow.Inputs); i++ {
		positionalArgs += "]"
	}

	var cmd = &cobra.Command{
		Use: fmt.Sprintf("%s %s", flow.Name, positionalArgs),
	}
	if flow.Description != "" {
		cmd.Short = flow.Description
		cmd.Long = flow.Description
	}

	flowKey := flow.Key

	if len(flow.Steps) > 0 {
		cmd.Run = func(cmd *cobra.Command, args []string) {
			p.app.UpdateLoggingConfiguration()

			output, err := p.app.RunFlowForKey(flowKey, args, flowApi.NewProvidedInputs())

			if err != nil {
				c := strings.Join(strings.Split(flowKey.String(), "."), " ")
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

			println(output)
		}
	}

	if rootCommand != nil {
		rootCommand.AddCommand(cmd)
	}

	log.WithFields(log.Fields{"prefix": flowKey.String()}).Debug("is a flow")

	for _, f := range flow.Flows {
		p.GenerateCommand(f, cmd)
	}

	flow.Command = cmd

	return cmd, nil
}

func (p *CobraAdapter) Flows() map[string]*Flow {
	return p.app.Flows()
}

func (p *CobraAdapter) GenerateAllFlags() {
	for _, flow := range p.Flows() {
		for _, input := range flow.ResolvedInputs {
			log.Debugf("Configuring flag and config key for flow %s's input: %s", flow.Key.String(), input.Name)

			flowConfig := flow.FlowConfig
			cmd := flow.Command
			var description string
			if input.Description != "" {
				description = input.Description
			} else {
				description = input.Name
			}

			var name string
			if input.FlowKey.String() == flow.Key.String() {
				name = input.Name
			} else {
				name = input.ShortName()
			}

			log.Debugf("short=%s, full=%s, name=%s, selected=%s", input.ShortName(), input.FullName, input.Name, name)

			flagName := strings.Replace(name, ".", "-", -1)

			var longerName string
			if input.FlowKey.ShortString() == flow.Key.ShortString() {
				longerName = input.ShortName()
			} else {
				longerName = fmt.Sprintf("%s.%s", flow.Key.ShortString(), input.ShortName())
			}

			if len(flowConfig.FlowConfigs) == 0 {
				cmd.Flags().StringP(flagName, "" /*string(input.Name[0])*/, "", description)
				//log.Debugf("Binding flag --%s to the config key %s", flagName, name)
				//viper.BindPFlag(name, cmd.Flags().Lookup(flagName))
				log.Debugf("Binding flag --%s of the command %s to the input %s with the config key %s", flagName, flow.Key.ShortString(), input.Name, longerName)
				viper.BindPFlag(longerName, cmd.Flags().Lookup(flagName))
			} else {
				cmd.PersistentFlags().StringP(flagName, "" /*string(input.Name[0])*/, "" /*default*/, description)
				//log.Debugf("Binding persistent flag --%s to the config key %s", flagName, name)
				//viper.BindPFlag(name, cmd.PersistentFlags().Lookup(flagName))
				log.Debugf("Binding persistent flag --%s to the config key %s", flagName, longerName)
				viper.BindPFlag(longerName, cmd.PersistentFlags().Lookup(flagName))
			}
		}
	}
}
