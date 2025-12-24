# Cronzee - Application Health Monitor

A lightweight, configurable health monitoring application written in Go that checks application endpoints and sends alerts when issues are detected.

## Features

- 🔍 **Endpoint Monitoring**: Monitor multiple HTTP/HTTPS endpoints
- ⚡ **Configurable Checks**: Set custom timeouts, expected status codes, and check intervals
- 🚨 **Smart Alerting**: Failure and recovery thresholds to avoid alert fatigue
- 📢 **Multiple Alert Channels**:
  - Generic Webhooks
  - Slack notifications
  - Email alerts (SMTP)
- 🎯 **Flexible Configuration**: YAML-based configuration
- 🔄 **Graceful Shutdown**: Proper signal handling
- 📊 **Detailed Logging**: Track all health check results

## Installation

### Prerequisites

- Go 1.21 or higher

### Build from Source

```bash
# Clone the repository
cd /Users/ashanmugaraja/ws-is/cronzee

# Download dependencies
go mod download

# Build the application
go build -o cronzee

# Run the application
./cronzee -config config.yaml
```

## Configuration

Create a `config.yaml` file with your monitoring configuration:

```yaml
check_interval: 30s

endpoints:
  - name: "My API"
    url: "https://api.example.com/health"
    method: "GET"
    timeout: 10s
    expected_status: 200
    failure_threshold: 3
    success_threshold: 2
    headers:
      Authorization: "Bearer YOUR_TOKEN"

alerting:
  enabled: true
  webhook_url: "https://your-webhook.com/alerts"
  slack_enabled: true
  slack_webhook: "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK"
```

### Configuration Options

#### Global Settings

- `check_interval`: How often to check all endpoints (e.g., `30s`, `1m`, `5m`)

#### Endpoint Configuration

- `name`: Friendly name for the endpoint
- `url`: Full URL to check
- `method`: HTTP method (default: `GET`)
- `timeout`: Request timeout (default: `10s`)
- `expected_status`: Expected HTTP status code (default: `200`)
- `failure_threshold`: Consecutive failures before marking unhealthy (default: `3`)
- `success_threshold`: Consecutive successes before marking healthy (default: `2`)
- `headers`: Custom HTTP headers (optional)

#### Alerting Configuration

- `enabled`: Enable/disable all alerts
- `webhook_url`: Generic webhook endpoint for custom integrations
- `slack_enabled`: Enable Slack notifications
- `slack_webhook`: Slack webhook URL
- `email_enabled`: Enable email alerts
- `email_config`: SMTP configuration for email alerts
- `custom_fields`: Additional fields to include in alerts

## Usage

### Basic Usage

```bash
# Run with default config file (config.yaml)
./cronzee

# Run with custom config file
./cronzee -config /path/to/config.yaml
```

### Running as a Service

#### systemd (Linux)

Create `/etc/systemd/system/cronzee.service`:

```ini
[Unit]
Description=Cronzee Health Monitor
After=network.target

[Service]
Type=simple
User=cronzee
WorkingDirectory=/opt/cronzee
ExecStart=/opt/cronzee/cronzee -config /opt/cronzee/config.yaml
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl enable cronzee
sudo systemctl start cronzee
sudo systemctl status cronzee
```

#### Docker

Create a `Dockerfile`:

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o cronzee

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/cronzee .
COPY config.yaml .
CMD ["./cronzee"]
```

Build and run:

```bash
docker build -t cronzee .
docker run -d --name cronzee -v $(pwd)/config.yaml:/root/config.yaml cronzee
```

## Alert Formats

### Webhook Payload

```json
{
  "subject": "[CRONZEE] Alert: My API is DOWN",
  "message": "Detailed error message...",
  "alert_type": "failure",
  "endpoint": {
    "name": "My API",
    "url": "https://api.example.com/health",
    "method": "GET"
  },
  "state": {
    "status": "unhealthy",
    "consecutive_failures": 3,
    "last_error": "request failed: context deadline exceeded",
    "response_time_ms": 10000,
    "last_check": "2025-12-16T11:20:00Z"
  },
  "timestamp": "2025-12-16T11:20:00Z"
}
```

### Slack Message

Cronzee sends formatted Slack messages with:
- Color-coded alerts (red for failures, green for recovery)
- Endpoint details
- Status and response time
- Error messages (if applicable)

## Monitoring Best Practices

1. **Set Appropriate Thresholds**: Use `failure_threshold` > 1 to avoid false positives
2. **Configure Timeouts**: Set realistic timeouts based on your endpoint's expected response time
3. **Use Health Endpoints**: Create dedicated `/health` endpoints that check critical dependencies
4. **Monitor Critical Paths**: Focus on user-facing and critical business endpoints
5. **Test Alerts**: Verify alert delivery before deploying to production

## Troubleshooting

### No Alerts Received

1. Check that `alerting.enabled` is `true`
2. Verify webhook URLs are correct and accessible
3. Check application logs for error messages
4. Test webhook endpoints manually

### False Positives

1. Increase `failure_threshold` to require more consecutive failures
2. Increase `timeout` if endpoints are slow
3. Check network connectivity between monitor and endpoints

### High Memory Usage

1. Reduce `check_interval` to check less frequently
2. Reduce number of monitored endpoints
3. Check for endpoint response size (large responses consume more memory)

## Development

### Running Tests

```bash
go test ./...
```

### Building for Different Platforms

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o cronzee-linux

# macOS
GOOS=darwin GOARCH=amd64 go build -o cronzee-macos

# Windows
GOOS=windows GOARCH=amd64 go build -o cronzee.exe
```

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

MIT License - feel free to use this in your projects.

## Support

For issues, questions, or contributions, please open an issue on the repository.
