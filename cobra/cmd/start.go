/*
Copyright © 2020 author bigrain

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"

	"github.com/darunshen/go/chat"
	"github.com/spf13/cobra"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "start the server",
	Long:  `start the server.`,
	Run: func(cmd *cobra.Command, args []string) {
		host, err := netCmd.Flags().GetString("host")
		if err != nil {
			fmt.Printf("get host error : %v\n", err)
		}
		port, err := netCmd.Flags().GetInt("port")
		if err != nil {
			fmt.Printf("get port error : %v\n", port)
		}
		fmt.Printf("start called to %v:%v\n", host, port)
		chat.Start(host, port)
	},
}

func init() {
	serverCmd.AddCommand(startCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// startCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// startCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
