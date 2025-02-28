package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/thannaske/s3usage/pkg/models"
)

// DB represents the database connection
type DB struct {
	*sql.DB
}

// NewDB creates a new database connection
func NewDB(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

// InitDB initializes the database tables
func (db *DB) InitDB() error {
	// Create bucket_usage table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS bucket_usage (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bucket_name TEXT NOT NULL,
			size_bytes INTEGER NOT NULL,
			object_count INTEGER NOT NULL,
			timestamp DATETIME NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	// Create monthly_averages table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS monthly_averages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bucket_name TEXT NOT NULL,
			year INTEGER NOT NULL,
			month INTEGER NOT NULL,
			avg_size_bytes REAL NOT NULL,
			avg_object_count REAL NOT NULL,
			data_points INTEGER NOT NULL,
			UNIQUE(bucket_name, year, month)
		)
	`)
	if err != nil {
		return err
	}

	// Create an index on bucket_name and timestamp for faster queries
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_bucket_usage_name_time 
		ON bucket_usage(bucket_name, timestamp)
	`)
	return err
}

// StoreBucketUsage stores the bucket usage data in the database
func (db *DB) StoreBucketUsage(usage models.BucketUsage) error {
	_, err := db.Exec(`
		INSERT INTO bucket_usage (bucket_name, size_bytes, object_count, timestamp)
		VALUES (?, ?, ?, ?)
	`, usage.BucketName, usage.SizeBytes, usage.ObjectCount, usage.Timestamp)
	return err
}

// GetBucketUsage retrieves the usage data for a specific bucket
func (db *DB) GetBucketUsage(bucketName string, startTime, endTime time.Time) ([]models.BucketUsage, error) {
	rows, err := db.Query(`
		SELECT id, bucket_name, size_bytes, object_count, timestamp
		FROM bucket_usage
		WHERE bucket_name = ? AND timestamp BETWEEN ? AND ?
		ORDER BY timestamp
	`, bucketName, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usages []models.BucketUsage
	for rows.Next() {
		var u models.BucketUsage
		if err := rows.Scan(&u.ID, &u.BucketName, &u.SizeBytes, &u.ObjectCount, &u.Timestamp); err != nil {
			return nil, err
		}
		usages = append(usages, u)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return usages, nil
}

// CalculateMonthlyAverages calculates the monthly averages for all buckets
func (db *DB) CalculateMonthlyAverages(year, month int) error {
	startDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, 0).Add(-time.Second)

	// Get all unique bucket names for the given month
	bucketRows, err := db.Query(`
		SELECT DISTINCT bucket_name
		FROM bucket_usage
		WHERE timestamp BETWEEN ? AND ?
	`, startDate, endDate)
	if err != nil {
		return err
	}
	defer bucketRows.Close()

	var buckets []string
	for bucketRows.Next() {
		var bucket string
		if err := bucketRows.Scan(&bucket); err != nil {
			return err
		}
		buckets = append(buckets, bucket)
	}

	// For each bucket, calculate the average
	for _, bucketName := range buckets {
		// Calculate averages
		var avgSize float64
		var avgCount float64
		var dataPoints int
		err := db.QueryRow(`
			SELECT AVG(size_bytes), AVG(object_count), COUNT(*)
			FROM bucket_usage
			WHERE bucket_name = ? AND timestamp BETWEEN ? AND ?
		`, bucketName, startDate, endDate).Scan(&avgSize, &avgCount, &dataPoints)
		if err != nil {
			return err
		}

		// Insert or update the monthly average
		_, err = db.Exec(`
			INSERT INTO monthly_averages 
			(bucket_name, year, month, avg_size_bytes, avg_object_count, data_points)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(bucket_name, year, month) 
			DO UPDATE SET 
				avg_size_bytes = excluded.avg_size_bytes,
				avg_object_count = excluded.avg_object_count,
				data_points = excluded.data_points
		`, bucketName, year, month, avgSize, avgCount, dataPoints)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetMonthlyAverage gets the monthly average for a specific bucket
func (db *DB) GetMonthlyAverage(bucketName string, year, month int) (*models.MonthlyBucketAverage, error) {
	var avg models.MonthlyBucketAverage
	err := db.QueryRow(`
		SELECT bucket_name, year, month, avg_size_bytes, avg_object_count, data_points
		FROM monthly_averages
		WHERE bucket_name = ? AND year = ? AND month = ?
	`, bucketName, year, month).Scan(
		&avg.BucketName, &avg.Year, &avg.Month,
		&avg.AvgSizeBytes, &avg.AvgObjectCount, &avg.DataPoints,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no data available for bucket %s in %d-%02d", bucketName, year, month)
	}
	if err != nil {
		return nil, err
	}
	return &avg, nil
}

// GetAllMonthlyAverages gets all monthly averages for a specific month
func (db *DB) GetAllMonthlyAverages(year, month int) ([]models.MonthlyBucketAverage, error) {
	rows, err := db.Query(`
		SELECT bucket_name, year, month, avg_size_bytes, avg_object_count, data_points
		FROM monthly_averages
		WHERE year = ? AND month = ?
		ORDER BY bucket_name
	`, year, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var averages []models.MonthlyBucketAverage
	for rows.Next() {
		var avg models.MonthlyBucketAverage
		if err := rows.Scan(
			&avg.BucketName, &avg.Year, &avg.Month,
			&avg.AvgSizeBytes, &avg.AvgObjectCount, &avg.DataPoints,
		); err != nil {
			return nil, err
		}
		averages = append(averages, avg)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return averages, nil
}

// PruneOldData removes individual bucket usage data points from months that have
// already been aggregated into monthly averages.
// It keeps data from the current month and any months that don't have averages calculated.
func (db *DB) PruneOldData() (int64, error) {
	// Get the current date
	now := time.Now()

	// Start of the current month
	currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	// Begin a transaction
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if not committed

	// Get a list of months for which we have monthly averages
	rows, err := tx.Query(`
		SELECT DISTINCT year, month 
		FROM monthly_averages
		ORDER BY year, month
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to query monthly averages: %w", err)
	}
	defer rows.Close()

	var completedMonths []time.Time
	for rows.Next() {
		var year, month int
		if err := rows.Scan(&year, &month); err != nil {
			return 0, fmt.Errorf("failed to scan monthly average row: %w", err)
		}

		// Convert to time.Time for easier comparison
		monthStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)

		// Only include months that are completed (before the current month)
		if monthStart.Before(currentMonthStart) {
			completedMonths = append(completedMonths, monthStart)
		}
	}

	if err = rows.Err(); err != nil {
		return 0, fmt.Errorf("error iterating monthly average rows: %w", err)
	}

	if len(completedMonths) == 0 {
		// No pruning needed
		return 0, nil
	}

	// For each completed month, delete the individual data points
	var totalDeleted int64 = 0
	for _, monthStart := range completedMonths {
		// End of the month
		monthEnd := monthStart.AddDate(0, 1, 0).Add(-time.Second)

		// Delete all individual data points for this month
		result, err := tx.Exec(`
			DELETE FROM bucket_usage
			WHERE timestamp >= ? AND timestamp <= ?
		`, monthStart, monthEnd)
		if err != nil {
			return 0, fmt.Errorf("failed to delete data points for %s: %w",
				monthStart.Format("2006-01"), err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("failed to get rows affected: %w", err)
		}

		totalDeleted += rowsAffected
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return totalDeleted, nil
}
