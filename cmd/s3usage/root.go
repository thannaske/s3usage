package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/thannaske/s3usage/pkg/models"
)

var (
	cfgFile   string
	config    models.Config
	defaultDB = filepath.Join(os.Getenv("HOME"), ".s3usage.db")
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "s3usage",
	Short: "S3 bucket usage monitor for Ceph",
	Long: `A CLI tool to monitor and track usage statistics for S3 buckets in Ceph.
It collects and stores usage data in a SQLite database and provides
commands to query historical usage information.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.s3usage.yaml)")
	rootCmd.PersistentFlags().StringVar(&config.S3Endpoint, "endpoint", "", "S3 endpoint URL")
	rootCmd.PersistentFlags().StringVar(&config.S3AccessKey, "access-key", "", "S3 access key")
	rootCmd.PersistentFlags().StringVar(&config.S3SecretKey, "secret-key", "", "S3 secret key")
	rootCmd.PersistentFlags().StringVar(&config.S3Region, "region", "default", "S3 region")
	rootCmd.PersistentFlags().StringVar(&config.DBPath, "db", defaultDB, "SQLite database path")
}

// initConfig reads in config file if set.
func initConfig() {
	// If config file path is provided, use it
	if cfgFile != "" {
		// TODO: Read config from file
		// For now, we'll just use command line flags
	}

	// Environment variables can override config
	if os.Getenv("S3_ENDPOINT") != "" {
		config.S3Endpoint = os.Getenv("S3_ENDPOINT")
	}
	if os.Getenv("S3_ACCESS_KEY") != "" {
		config.S3AccessKey = os.Getenv("S3_ACCESS_KEY")
	}
	if os.Getenv("S3_SECRET_KEY") != "" {
		config.S3SecretKey = os.Getenv("S3_SECRET_KEY")
	}
	if os.Getenv("S3_REGION") != "" {
		config.S3Region = os.Getenv("S3_REGION")
	}
	if os.Getenv("S3_DB_PATH") != "" {
		config.DBPath = os.Getenv("S3_DB_PATH")
	}
}
