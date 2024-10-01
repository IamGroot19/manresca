/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package estimate

import (
	"fmt"

	"github.com/spf13/cobra"
)

// estimateCmd represents the estimate command
var EstimateCmd = &cobra.Command{
	Use:   "estimate",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Estimate called with verbosity %d for the filepath %s\n", reportVerbosity, manifestPath)
		ProcessManifest(manifestPath)
	},
}

var (
	reportVerbosity int
	manifestPath    string
)

func init() {

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	EstimateCmd.PersistentFlags().IntVarP(&reportVerbosity, "verbosity", "v", 0, "Provide the verbosity at which report needs to be printed: \n 0: Just print the Object Name, Kind, CPU (req,lim), Mem (Req, Lim) \n 1: Print things menioned in 0 along with a column mentioning replica count (wherever applicable)")

	EstimateCmd.PersistentFlags().StringVarP(&manifestPath, "filepath", "f", "rendered.yml", "Provide the path to the rendered manifest file (i.e this filel would have output contents of `helm template <chart path> -f <values-file-path>`)")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// estimateCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
