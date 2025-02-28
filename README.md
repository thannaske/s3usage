# S3Usage - S3 Bucket Usage Monitor for Ceph

S3Usage is a command-line application that monitors storage usage of S3 buckets in a Ceph cluster. It uses Ceph's RGW Admin Ops API to efficiently retrieve bucket statistics, stores the data in a SQLite database, calculates monthly averages, and provides commands to query historical usage information.

## Features

- Connect to Ceph S3 RGW Admin API to efficiently retrieve bucket usage statistics
- Store usage data in a SQLite database
- Calculate monthly average bucket usage
- Display monthly usage for all buckets
- Query historical usage for specific buckets
- Prune old data points while preserving monthly statistics

## Implementation Details

This tool uses two different APIs:
- Standard S3 API to list the buckets (as a fallback)
- Ceph RGW Admin Ops API to fetch bucket statistics efficiently

The Admin API requests are properly signed using AWS Signature v4 authentication, which is required by Ceph RGW. Using the Admin Ops API provides significant performance improvements over listing all objects in buckets, making this tool suitable for monitoring Ceph S3 deployments with many large buckets.

## Authentication Requirements

The tool uses proper AWS Signature v4 authentication for Admin API requests, identical to the authentication used by the `radosgw-admin` CLI tool. This requires:

1. A Ceph user with administrative privileges
2. The access and secret keys for that user
3. The correct Ceph RGW endpoint URL

## Installation

### From Source

```bash
git clone https://github.com/thannaske/s3usage.git
cd s3usage
go build -o s3usage ./cmd/s3usage
# Optionally, move to a directory in your PATH
sudo mv s3usage /usr/local/bin/
```

## Usage

### Environment Variables

You can configure S3Usage using environment variables:

- `S3_ENDPOINT`: Ceph S3 endpoint URL (should be the RGW API endpoint)
- `S3_ACCESS_KEY`: S3 access key (requires admin privileges for RGW Admin API)
- `S3_SECRET_KEY`: S3 secret key
- `S3_REGION`: S3 region (default: "default")
- `S3_DB_PATH`: Path to SQLite database (default: `~/.s3usage.db`)

### Required Permissions

The access key used must have administrative privileges on the Ceph RGW to access the Admin API endpoints. You can create a user with the appropriate permissions using:

```bash
radosgw-admin user create --uid=s3usage --display-name="S3 Usage Monitor" --caps="buckets=*;users=*;usage=*;metadata=*;zone=*"
radosgw-admin key create --uid=s3usage --key-type=s3 --gen-access-key --gen-secret
```

### Collecting Usage Data

To collect bucket usage data and store it in the database, use the `collect` command:

```bash
s3usage collect --endpoint=https://s3.example.com --access-key=YOUR_ACCESS_KEY --secret-key=YOUR_SECRET_KEY
```

Or using environment variables:

```bash
export S3_ENDPOINT=https://s3.example.com
export S3_ACCESS_KEY=YOUR_ACCESS_KEY
export S3_SECRET_KEY=YOUR_SECRET_KEY
s3usage collect
```

This command is meant to be scheduled via cron to collect data regularly.

### Monthly Usage Report

To display the monthly average usage for all buckets:

```bash
s3usage list --year=2025 --month=2
```

If no year/month is specified, the previous month's data is shown.

### Bucket Usage History

To view historical usage data for a specific bucket:

```bash
s3usage history my-bucket-name
```

This shows a year's worth of historical data for the specified bucket.

### Pruning Old Data

To clean up individual data points from months that have already been aggregated into monthly averages:

```bash
s3usage prune
```

This will prompt for confirmation before deleting data. To skip the confirmation:

```bash
s3usage prune --confirm
```

The prune command only removes data points from completed months that already have monthly averages calculated. It preserves:
- All monthly average statistics
- Data points from the current month
- Data points from months without calculated averages

This helps keep the database size manageable over time without losing valuable statistics.

## Cron Setup

To collect data daily, add a cron job:

```bash
# Edit crontab
crontab -e

# Add this line to run daily at 23:45
45 23 * * * /usr/local/bin/s3usage collect --endpoint=https://s3.example.com --access-key=YOUR_ACCESS_KEY --secret-key=YOUR_SECRET_KEY
```

## Troubleshooting

If you encounter authentication issues:

1. Verify your user has the correct admin capabilities
2. Ensure your endpoint URL is correct (it should point to the RGW API endpoint)
3. Check that you're using the correct access and secret keys
4. Verify the region setting matches your Ceph configuration

## License

MIT 