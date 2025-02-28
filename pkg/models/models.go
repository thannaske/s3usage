package models

import (
	"time"
)

// BucketUsage represents the disk usage for a single bucket at a specific point in time
type BucketUsage struct {
	ID        int64     `json:"id"`
	BucketName string    `json:"bucket_name"`
	SizeBytes  int64     `json:"size_bytes"`
	ObjectCount int64    `json:"object_count"`
	Timestamp  time.Time `json:"timestamp"`
}

// MonthlyBucketAverage represents the average disk usage for a bucket over a month
type MonthlyBucketAverage struct {
	BucketName   string  `json:"bucket_name"`
	Year         int     `json:"year"`
	Month        int     `json:"month"`
	AvgSizeBytes float64 `json:"avg_size_bytes"`
	AvgObjectCount float64 `json:"avg_object_count"`
	DataPoints   int     `json:"data_points"`
}

// Config represents the application configuration
type Config struct {
	S3Endpoint  string `json:"s3_endpoint"`
	S3AccessKey string `json:"s3_access_key"`
	S3SecretKey string `json:"s3_secret_key"`
	S3Region    string `json:"s3_region"`
	DBPath      string `json:"db_path"`
} 