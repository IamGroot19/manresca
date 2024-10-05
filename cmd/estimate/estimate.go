package estimate

import (
	"fmt"

	"github.com/spf13/cobra"
)

// estimateCmd represents the estimate command
var EstimateCmd = &cobra.Command{
	Use:   "estimate",
	Short: "Estimate resources required for deploying this helm chart",
	Long: `This command estimates & prints a tabular summary of resources needed for 
	deploying/applying a helm chart. Currently, only resources estimated are CPU & RAM.
	The only types which are parsed & summarised are Deployment, Statefulset, Job and Pod`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Estimate called with verbosity %d for the filepath %s\n", reportVerbosity, manifestPath)
		ProcessManifest(manifestPath, reportVerbosity)
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

	EstimateCmd.PersistentFlags().StringVarP(&manifestPath, "filepath", "f", "rendered.yml", "Provide the path to the rendered manifest file\n(i.e this filel would have output contents of 'helm template <chart path> -f <values-file-path>')\n")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// estimateCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
