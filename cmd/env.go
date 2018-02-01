// Copyright © 2016 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	env "github.com/mumoshu/variant/cli/env"
	subcommands "github.com/mumoshu/variant/cmd/env"
)

var EnvCmd = &cobra.Command{
	Use:   "env",
	Short: "Print currently selected environment",
	Long: `Print currently selected environment. The environment can be via the command "var env set" or "var env switch".

Example:
var env switch dev
var env #=> Prints "dev"
`,
	Run: func(cmd *cobra.Command, args []string) {
		env, err := env.Get()
		if err != nil {
			panic(err)
		}
		fmt.Println(env)
	},
}

func init() {
	EnvCmd.AddCommand(subcommands.SetCmd)
}
