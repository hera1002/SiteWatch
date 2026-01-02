package worker

import (
	"crypto/tls"
	"net/url"
	"time"
)

// SSLCertInfo holds SSL certificate information
type SSLCertInfo struct {
	Expiry          time.Time
	DaysToExpiry    int
	ExpiringSoon    bool
	IsHTTPS         bool
	Error           string
}

// CheckSSLCertificate checks the SSL certificate expiry for a given URL
func CheckSSLCertificate(urlStr string, warningDays int) SSLCertInfo {
	info := SSLCertInfo{
		IsHTTPS: false,
	}

	// Parse URL to check if it's HTTPS
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		info.Error = "Invalid URL"
		return info
	}

	// Only check HTTPS URLs
	if parsedURL.Scheme != "https" {
		return info
	}

	info.IsHTTPS = true

	// Extract hostname
	hostname := parsedURL.Hostname()
	if hostname == "" {
		info.Error = "Invalid hostname"
		return info
	}

	// Add default port if not specified
	address := hostname + ":443"
	if parsedURL.Port() != "" {
		address = hostname + ":" + parsedURL.Port()
	}

	// Connect with timeout and get certificate
	conn, err := tls.Dial("tcp", address, &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         hostname,
	})
	if err != nil {
		info.Error = "Failed to connect: " + err.Error()
		return info
	}
	defer conn.Close()

	// Get certificate chain
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		info.Error = "No certificates found"
		return info
	}

	// Get the leaf certificate (first in chain)
	cert := certs[0]
	info.Expiry = cert.NotAfter

	// Calculate days to expiry
	now := time.Now()
	duration := cert.NotAfter.Sub(now)
	info.DaysToExpiry = int(duration.Hours() / 24)

	// Check if expiring within configured warning days
	info.ExpiringSoon = info.DaysToExpiry <= warningDays && info.DaysToExpiry >= 0

	return info
}
