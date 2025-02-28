package ceph

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/thannaske/s3usage/pkg/models"
)

// S3Client represents a client for interacting with Ceph S3
type S3Client struct {
	client      *s3.Client
	adminClient *http.Client
	endpoint    string
	accessKey   string
	secretKey   string
	region      string
}

// NewS3Client creates a new Ceph S3 client
func NewS3Client(cfg models.Config) (*S3Client, error) {
	// Create custom resolver to use the Ceph endpoint
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               cfg.S3Endpoint,
			SigningRegion:     cfg.S3Region,
			HostnameImmutable: true,
		}, nil
	})

	// Create AWS credentials with the provided access and secret keys
	creds := credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, "")

	// Load the AWS SDK configuration with custom resolver and credentials
	awsCfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithCredentialsProvider(creds),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithRegion(cfg.S3Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS SDK configuration: %w", err)
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(awsCfg)

	// Create an HTTP client for admin operations
	adminClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &S3Client{
		client:      s3Client,
		adminClient: adminClient,
		endpoint:    cfg.S3Endpoint,
		accessKey:   cfg.S3AccessKey,
		secretKey:   cfg.S3SecretKey,
		region:      cfg.S3Region,
	}, nil
}

// BucketStats represents the statistics of a bucket from Ceph RGW Admin API
type BucketStats struct {
	Bucket      string `json:"bucket"`
	Usage       Usage  `json:"usage"`
	OwnerID     string `json:"id"`
	OwnerName   string `json:"owner"`
	Zonegroup   string `json:"zonegroup"`
	PlacementID string `json:"placement_rule"`
	Created     string `json:"creation_time"`
}

// Usage contains usage statistics for a bucket
type Usage struct {
	RgwMain struct {
		SizeKB       int64 `json:"size_kb"`
		SizeKBActual int64 `json:"size_kb_actual"`
		NumObjects   int64 `json:"num_objects"`
	} `json:"rgw.main"`
}

// executeSignedRequest executes an API request with proper AWS v4 signature
func (c *S3Client) executeSignedRequest(ctx context.Context, method, path string, queryParams url.Values, reqBody []byte) ([]byte, error) {
	// Parse the endpoint URL
	parsedURL, err := url.Parse(c.endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse endpoint URL: %w", err)
	}

	// Set the path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	parsedURL.Path = path

	// Add query parameters if any
	if queryParams != nil {
		parsedURL.RawQuery = queryParams.Encode()
	}

	// Prepare the request
	var bodyReader io.ReadSeeker
	if reqBody != nil {
		bodyReader = bytes.NewReader(reqBody)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	req, err := http.NewRequestWithContext(ctx, method, parsedURL.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Calculate sha256 hash of the request body
	var hashBytes []byte
	if reqBody != nil {
		h := sha256.New()
		h.Write(reqBody)
		hashBytes = h.Sum(nil)
	} else {
		h := sha256.New()
		hashBytes = h.Sum(nil)
	}
	payloadHash := hex.EncodeToString(hashBytes)

	// Create credentials
	creds := aws.Credentials{
		AccessKeyID:     c.accessKey,
		SecretAccessKey: c.secretKey,
	}

	// Sign the request - try both service names 'rgw' and 's3'
	// Ceph documentation mentions it should be s3, but some deployments may use rgw
	signer := v4.NewSigner()

	// Add the payload hash to the request headers
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	// Try with service name 's3' first (most common)
	err = signer.SignHTTP(ctx, creds, req, payloadHash, "s3", c.region, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	// Debug output for troubleshooting
	fmt.Printf("Executing request: %s %s\n", method, req.URL.String())
	fmt.Printf("Authorization: %s\n", req.Header.Get("Authorization"))

	// Execute the request
	resp, err := c.adminClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read the full response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check if the response was successful
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// GetBuckets retrieves the list of buckets using the Admin API
func (c *S3Client) GetBuckets(ctx context.Context) ([]string, error) {
	// Call the admin API to list buckets
	respBody, err := c.executeSignedRequest(ctx, "GET", "/admin/bucket", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets with Admin API: %w", err)
	}

	// Decode the response - it should be a simple array of strings
	var bucketList []string
	if err := json.Unmarshal(respBody, &bucketList); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return bucketList, nil
}

// GetBucketUsage retrieves the usage statistics for a bucket using the Ceph RGW Admin API
func (c *S3Client) GetBucketUsage(ctx context.Context, bucketName string) (*models.BucketUsage, error) {
	// Prepare query parameters
	queryParams := url.Values{}
	queryParams.Set("bucket", bucketName)
	queryParams.Set("stats", "true")

	// Call the admin API to get bucket stats
	respBody, err := c.executeSignedRequest(ctx, "GET", "/admin/bucket", queryParams, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket stats: %w", err)
	}

	// Decode the response
	var bucketStats BucketStats
	if err := json.Unmarshal(respBody, &bucketStats); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Create bucket usage object
	usage := &models.BucketUsage{
		BucketName:  bucketName,
		SizeBytes:   bucketStats.Usage.RgwMain.SizeKB * 1024, // Convert KB to bytes
		ObjectCount: bucketStats.Usage.RgwMain.NumObjects,
		Timestamp:   time.Now().UTC(),
	}

	return usage, nil
}

// GetAllBucketsUsage retrieves usage statistics for all buckets
func (c *S3Client) GetAllBucketsUsage(ctx context.Context) ([]models.BucketUsage, error) {
	// Get list of buckets
	buckets, err := c.GetBuckets(ctx)
	if err != nil {
		return nil, err
	}

	// Get usage stats for each bucket
	var usages []models.BucketUsage
	for _, bucketName := range buckets {
		fmt.Printf("Collecting statistics for bucket: %s\n", bucketName)
		usage, err := c.GetBucketUsage(ctx, bucketName)
		if err != nil {
			// Log error but continue with other buckets
			fmt.Printf("Error getting usage for bucket %s: %v\n", bucketName, err)
			continue
		}
		usages = append(usages, *usage)
	}

	return usages, nil
}
