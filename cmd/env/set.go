// Copyright © 2016 Yusuke KUOKA <ykuoka@gmail.com>
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

	"github.com/mumoshu/variant/pkg/cli/env"
)

var SetCmd = &cobra.Command{
	Use:     "set <environment name>",
	Aliases: []string{"switch", "use"},
	Short:   "Switch to another environment",
	Long: `Switch to another environment.

Environments may be one of those: dev(elopment), stg/staging, prod(uction) or etc.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := env.Set(args[0]); err != nil {
			panic(err)
		}

		env, err := env.Get()
		if err != nil {
			panic(err)
		}
		fmt.Printf("Environment is now: %s", env)
	},
}
