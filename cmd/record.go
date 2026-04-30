package cmd

import (
	"fmt"
	"tree6/internal/recorder"

	"github.com/spf13/cobra"
)

var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "Record user interactions",
	Long:  `Record keyboard and mouse interactions, along with active window information and screenshots.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Recording command started.")
		fmt.Println("Press Ctrl+Shift+B to start recording.")
		fmt.Println("Press Ctrl+Shift+E to stop recording.")

		r := recorder.NewRecorder()
		r.Start()
	},
}

func init() {
	rootCmd.AddCommand(recordCmd)
}
