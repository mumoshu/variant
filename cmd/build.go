// Copyright © 2018 Yusuke KUOKA <ykuoka@gmail.com>
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
	"github.com/spf13/cobra"

	"fmt"
	"github.com/mumoshu/variant/pkg/load"
	"os"
)

var BuildCmd = &cobra.Command{
	Use:   "build VARIANTFILE",
	Short: "Create a single executable from the Variantfile",
	Long:  `Create a single executable from the Variantfile`,
	RunE: func(cmd *cobra.Command, args []string) error {
		taskDef, err := load.File(args[0])
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "%#v", taskDef)
		return nil
	},
}
