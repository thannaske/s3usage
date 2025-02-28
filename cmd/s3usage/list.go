package main

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/thannaske/s3usage/pkg/db"
)

var (
	year  int
	month int
)

// formatSize converts bytes to a human-readable format
func formatSize(bytes float64) string {
	const (
		_          = iota
		KB float64 = 1 << (10 * iota)
		MB
		GB
		TB
		PB
	)

	unit := ""
	value := bytes

	switch {
	case bytes >= PB:
		unit = "PB"
		value = bytes / PB
	case bytes >= TB:
		unit = "TB"
		value = bytes / TB
	case bytes >= GB:
		unit = "GB"
		value = bytes / GB
	case bytes >= MB:
		unit = "MB"
		value = bytes / MB
	case bytes >= KB:
		unit = "KB"
		value = bytes / KB
	default:
		unit = "bytes"
	}

	if unit == "bytes" {
		return fmt.Sprintf("%.0f %s", value, unit)
	}
	return fmt.Sprintf("%.2f %s", value, unit)
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List monthly bucket usage",
	Long:  `Display monthly average usage statistics for all buckets.`,
	Run: func(cmd *cobra.Command, args []string) {
		// If no year/month specified, use previous month
		now := time.Now()
		if year == 0 {
			if now.Month() == 1 {
				year = now.Year() - 1
				month = 12
			} else {
				year = now.Year()
				month = int(now.Month()) - 1
			}
		}
		if month == 0 {
			month = int(now.Month())
		}

		// Validate month
		if month < 1 || month > 12 {
			fmt.Println("Error: Month must be between 1 and 12.")
			return
		}

		// Initialize the database
		database, err := db.NewDB(config.DBPath)
		if err != nil {
			fmt.Printf("Error connecting to database: %v\n", err)
			return
		}
		defer database.Close()

		// Get monthly averages
		averages, err := database.GetAllMonthlyAverages(year, month)
		if err != nil {
			fmt.Printf("Error retrieving monthly averages: %v\n", err)
			return
		}

		if len(averages) == 0 {
			fmt.Printf("No data available for %d-%02d\n", year, month)
			return
		}

		// Sort by size (largest first)
		sort.Slice(averages, func(i, j int) bool {
			return averages[i].AvgSizeBytes > averages[j].AvgSizeBytes
		})

		// Print the results
		fmt.Printf("Monthly Average Usage for %d-%02d\n\n", year, month)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.TabIndent)
		fmt.Fprintln(w, "Bucket\tSize\tObjects\tSamples")
		fmt.Fprintln(w, "------\t----\t-------\t-------")

		for _, avg := range averages {
			fmt.Fprintf(w, "%s\t%s\t%d\t%d\n",
				avg.BucketName,
				formatSize(avg.AvgSizeBytes),
				int(avg.AvgObjectCount),
				avg.DataPoints,
			)
		}
		w.Flush()
	},
}

var historyCmd = &cobra.Command{
	Use:   "history [bucket-name]",
	Short: "Show usage history for a bucket",
	Long:  `Display historical usage data for a specific bucket.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		bucketName := args[0]

		// Initialize the database
		database, err := db.NewDB(config.DBPath)
		if err != nil {
			fmt.Printf("Error connecting to database: %v\n", err)
			return
		}
		defer database.Close()

		// Calculate date range
		now := time.Now()
		startTime := time.Date(now.Year()-1, now.Month(), 1, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, time.UTC)

		// Get usage history
		usages, err := database.GetBucketUsage(bucketName, startTime, endTime)
		if err != nil {
			fmt.Printf("Error retrieving usage history: %v\n", err)
			return
		}

		if len(usages) == 0 {
			fmt.Printf("No usage data available for bucket %s\n", bucketName)
			return
		}

		// Print the results
		fmt.Printf("Usage History for Bucket: %s\n\n", bucketName)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.TabIndent)
		fmt.Fprintln(w, "Date\tSize\tObjects")
		fmt.Fprintln(w, "----\t----\t-------")

		for _, usage := range usages {
			fmt.Fprintf(w, "%s\t%s\t%d\n",
				usage.Timestamp.Format("2006-01-02 15:04:05"),
				formatSize(float64(usage.SizeBytes)),
				usage.ObjectCount,
			)
		}
		w.Flush()
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(historyCmd)

	// Add flags to the list command
	listCmd.Flags().IntVar(&year, "year", 0, "Year to query (default: current year)")
	listCmd.Flags().IntVar(&month, "month", 0, "Month to query (1-12, default: current month)")
}
