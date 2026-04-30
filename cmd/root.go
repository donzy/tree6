package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tree6",
	Short: "A CLI tool for recording user interactions",
	Long:  `A command-line interface tool for recording keyboard and mouse interactions, along with active window information and screenshots.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
