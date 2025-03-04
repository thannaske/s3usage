package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/thannaske/s3usage/pkg/ceph"
	"github.com/thannaske/s3usage/pkg/db"
)

var collectCmd = &cobra.Command{
	Use:   "collect",
	Short: "Collect bucket usage data",
	Long:  `Collect usage data for all buckets and store it in the database.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Validate required parameters
		if config.S3Endpoint == "" || config.S3AccessKey == "" || config.S3SecretKey == "" {
			fmt.Println("Error: Missing required S3 credentials. Please provide --endpoint, --access-key, and --secret-key.")
			return
		}

		// Initialize the database
		database, err := db.NewDB(config.DBPath)
		if err != nil {
			fmt.Printf("Error connecting to database: %v\n", err)
			return
		}
		defer database.Close()

		err = database.InitDB()
		if err != nil {
			fmt.Printf("Error initializing database: %v\n", err)
			return
		}

		// Initialize the S3 client
		s3Client, err := ceph.NewS3Client(config)
		if err != nil {
			fmt.Printf("Error initializing S3 client: %v\n", err)
			return
		}

		// Get usage data for all buckets
		fmt.Println("Collecting bucket usage data...")
		usages, err := s3Client.GetAllBucketsUsage(context.Background())
		if err != nil {
			fmt.Printf("Error collecting bucket usage data: %v\n", err)
			return
		}

		// Store usage data in the database
		for _, usage := range usages {
			err = database.StoreBucketUsage(usage)
			if err != nil {
				fmt.Printf("Error storing usage data for bucket %s: %v\n", usage.BucketName, err)
				continue
			}
			fmt.Printf("Stored usage data for bucket %s: %d bytes, %d objects\n",
				usage.BucketName, usage.SizeBytes, usage.ObjectCount)
		}

		// Always calculate monthly averages every time we collect data
		now := time.Now()
		fmt.Println("Calculating monthly averages...")
		err = database.CalculateMonthlyAverages(now.Year(), int(now.Month()))
		if err != nil {
			fmt.Printf("Error calculating monthly averages: %v\n", err)
			return
		}
		fmt.Println("Monthly averages calculated successfully.")

		fmt.Println("Collection completed successfully.")
	},
}

func init() {
	rootCmd.AddCommand(collectCmd)
}
