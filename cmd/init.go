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

	"io/ioutil"
	"os"
)

var InitCmd = &cobra.Command{
	Use:   "init NAME",
	Short: "Create a Variant command",
	Long: `Create a Variant command with the specified NAME

Example:
cat <<EOF | variant init mycmd
tasks:
  script: |
    echo Hello Variant!
EOF

./mycmd
`,
	Run: func(cmd *cobra.Command, args []string) {
		bytes, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			panic(err)
		}
		header := "#!/usr/bin/env variant\n\n"
		bytes = append([]byte(header), bytes...)
		if err := ioutil.WriteFile(args[0], bytes, 0755); err != nil {
			panic(err)
		}
	},
}
