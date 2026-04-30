package cmd

import (
	"fmt"
	"os"
	"tree6/internal/recorder"

	"github.com/spf13/cobra"
)

var recordFlag bool

var rootCmd = &cobra.Command{
	Use:   "tree6",
	Short: "A CLI tool for recording user interactions",
	Long:  `A command-line interface tool for recording keyboard and mouse interactions, along with active window information and screenshots.`,
	Run: func(cmd *cobra.Command, args []string) {
		if recordFlag {
			runRecord()
		}
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.Flags().BoolVar(&recordFlag, "record", false, "Start recording user interactions")
}

func runRecord() {
	fmt.Println("Recording command started.")
	fmt.Println()
	fmt.Println("IMPORTANT: On macOS, this program requires accessibility permissions.")
	fmt.Println("If you don't see any keyboard events, please grant accessibility")
	fmt.Println("permissions in System Preferences > Privacy & Security > Accessibility")
	fmt.Println()
	fmt.Println("Hotkeys:")
	fmt.Println("  Ctrl+Alt+B = Start recording")
	fmt.Println("  Ctrl+Alt+E = Stop recording")
	fmt.Println("  Ctrl+C = Exit program")
	fmt.Println()

	r := recorder.NewRecorder()
	r.Start()
}
