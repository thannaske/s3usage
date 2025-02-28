package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/thannaske/s3usage/pkg/db"
)

var (
	// Flag to confirm pruning without prompting
	confirm bool
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Prune old bucket usage data",
	Long: `Remove individual bucket usage data points from months that have already 
been aggregated into monthly averages. This helps keep the database size manageable
over time while preserving the monthly average statistics.

Only data from completed months with calculated monthly averages are removed.
Data from the current month and any months without averages are preserved.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Initialize the database
		database, err := db.NewDB(config.DBPath)
		if err != nil {
			fmt.Printf("Error connecting to database: %v\n", err)
			return
		}
		defer database.Close()

		// If not confirmed, prompt the user
		if !confirm {
			fmt.Print("This will permanently delete individual data points from months that have " +
				"completed and have calculated monthly averages.\n" +
				"The monthly average statistics will be preserved.\n" +
				"Are you sure you want to continue? (y/N): ")

			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				fmt.Println("Pruning cancelled.")
				return
			}
		}

		// Perform the pruning operation
		fmt.Println("Pruning old data points...")
		rowsDeleted, err := database.PruneOldData()
		if err != nil {
			fmt.Printf("Error pruning old data: %v\n", err)
			os.Exit(1)
		}

		if rowsDeleted == 0 {
			fmt.Println("No data to prune. All data points are still needed or no monthly averages have been calculated yet.")
		} else {
			fmt.Printf("Successfully pruned %d data points from completed months.\n", rowsDeleted)
		}
	},
}

func init() {
	rootCmd.AddCommand(pruneCmd)

	// Add flags to the prune command
	pruneCmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm pruning without prompting")
}
