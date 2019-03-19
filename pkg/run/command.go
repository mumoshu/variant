package run

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type CobraApp struct {
	viperCfg *viper.Viper
	cobraCmd *cobra.Command
}

func (a *CobraApp) Run(args []string) error {
	a.cobraCmd.SetArgs(args)
	return a.cobraCmd.Execute()
}
