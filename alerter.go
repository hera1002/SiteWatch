package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"strings"
	"time"
)

// Alerter handles sending alerts through various channels
type Alerter struct {
	config *Alerting
}

// NewAlerter creates a new alerter
func NewAlerter(config *Alerting) *Alerter {
	return &Alerter{
		config: config,
	}
}

// SendFailureAlert sends an alert when an endpoint becomes unhealthy
func (a *Alerter) SendFailureAlert(endpoint Endpoint, state *EndpointState) {
	if !a.config.Enabled {
		return
	}

	message := fmt.Sprintf(
		"🔴 ALERT: Endpoint '%s' is UNHEALTHY\n\n"+
			"URL: %s\n"+
			"Status: %s\n"+
			"Consecutive Failures: %d\n"+
			"Last Error: %s\n"+
			"Last Check: %s\n"+
			"Response Time: %v",
		endpoint.Name,
		endpoint.URL,
		state.Status,
		state.ConsecutiveFailures,
		state.LastError,
		state.LastCheck.Format(time.RFC3339),
		state.ResponseTime,
	)

	subject := fmt.Sprintf("[CRONZEE] Alert: %s is DOWN", endpoint.Name)

	a.sendAlert(subject, message, "failure", endpoint, state)
	// 🔔 NEW: Teams alert
	if a.config.TeamsEnabled && a.config.TeamsWebhook != "" {
		a.sendTeamsAlert(endpoint,state)
	}
}

// SendRecoveryAlert sends an alert when an endpoint recovers
func (a *Alerter) SendRecoveryAlert(endpoint Endpoint, state *EndpointState) {
	if !a.config.Enabled {
		return
	}

	downtime := time.Since(state.LastStatusChange)
	message := fmt.Sprintf(
		"✅ RECOVERY: Endpoint '%s' is HEALTHY\n\n"+
			"URL: %s\n"+
			"Status: %s\n"+
			"Downtime: %v\n"+
			"Response Time: %v\n"+
			"Last Check: %s",
		endpoint.Name,
		endpoint.URL,
		state.Status,
		downtime.Round(time.Second),
		state.ResponseTime,
		state.LastCheck.Format(time.RFC3339),
	)

	subject := fmt.Sprintf("[CRONZEE] Recovery: %s is UP", endpoint.Name)

	a.sendAlert(subject, message, "recovery", endpoint, state)
}

// sendAlert sends alerts through configured channels
func (a *Alerter) sendAlert(subject, message, alertType string, endpoint Endpoint, state *EndpointState) {
	// Send webhook alert
	if a.config.WebhookURL != "" {
		go a.sendWebhookAlert(subject, message, alertType, endpoint, state)
	}

	// Send Slack alert
	if a.config.SlackEnabled && a.config.SlackWebhook != "" {
		go a.sendSlackAlert(subject, message, alertType, endpoint, state)
	}

	// Send email alert
	if a.config.EmailEnabled {
		go a.sendEmailAlert(subject, message)
	}
}

// sendWebhookAlert sends a generic webhook alert
func (a *Alerter) sendWebhookAlert(subject, message, alertType string, endpoint Endpoint, state *EndpointState) {
	payload := map[string]interface{}{
		"subject":    subject,
		"message":    message,
		"alert_type": alertType,
		"endpoint": map[string]interface{}{
			"name":   endpoint.Name,
			"url":    endpoint.URL,
			"method": endpoint.Method,
		},
		"state": map[string]interface{}{
			"status":               string(state.Status),
			"consecutive_failures": state.ConsecutiveFailures,
			"last_error":           state.LastError,
			"response_time_ms":     state.ResponseTime.Milliseconds(),
			"last_check":           state.LastCheck.Format(time.RFC3339),
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	// Add custom fields
	for key, value := range a.config.CustomFields {
		payload[key] = value
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal webhook payload: %v", err)
		return
	}

	resp, err := http.Post(a.config.WebhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Failed to send webhook alert: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("Webhook alert sent successfully for endpoint: %s", endpoint.Name)
	} else {
		log.Printf("Webhook alert failed with status code: %d", resp.StatusCode)
	}
}

// sendSlackAlert sends an alert to Slack
func (a *Alerter) sendSlackAlert(subject, message, alertType string, endpoint Endpoint, state *EndpointState) {
	color := "danger"
	emoji := "🔴"
	if alertType == "recovery" {
		color = "good"
		emoji = "✅"
	}

	payload := map[string]interface{}{
		"text": fmt.Sprintf("%s %s", emoji, subject),
		"attachments": []map[string]interface{}{
			{
				"color": color,
				"fields": []map[string]interface{}{
					{
						"title": "Endpoint",
						"value": endpoint.Name,
						"short": true,
					},
					{
						"title": "URL",
						"value": endpoint.URL,
						"short": true,
					},
					{
						"title": "Status",
						"value": string(state.Status),
						"short": true,
					},
					{
						"title": "Response Time",
						"value": fmt.Sprintf("%v", state.ResponseTime),
						"short": true,
					},
				},
				"footer": "Cronzee Health Monitor",
				"ts":     time.Now().Unix(),
			},
		},
	}

	if state.LastError != "" {
		attachments := payload["attachments"].([]map[string]interface{})
		attachments[0]["fields"] = append(attachments[0]["fields"].([]map[string]interface{}), map[string]interface{}{
			"title": "Error",
			"value": state.LastError,
			"short": false,
		})
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal Slack payload: %v", err)
		return
	}

	resp, err := http.Post(a.config.SlackWebhook, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Failed to send Slack alert: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("Slack alert sent successfully for endpoint: %s", endpoint.Name)
	} else {
		log.Printf("Slack alert failed with status code: %d", resp.StatusCode)
	}
}

// sendEmailAlert sends an email alert
func (a *Alerter) sendEmailAlert(subject, message string) {
	if a.config.EmailConfig.SMTPHost == "" {
		log.Println("Email SMTP host not configured")
		return
	}

	auth := smtp.PlainAuth(
		"",
		a.config.EmailConfig.Username,
		a.config.EmailConfig.Password,
		a.config.EmailConfig.SMTPHost,
	)

	to := strings.Join(a.config.EmailConfig.To, ",")
	
	emailBody := fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"\r\n"+
			"%s\r\n",
		a.config.EmailConfig.From,
		to,
		subject,
		message,
	)

	addr := fmt.Sprintf("%s:%d", a.config.EmailConfig.SMTPHost, a.config.EmailConfig.SMTPPort)
	
	err := smtp.SendMail(
		addr,
		auth,
		a.config.EmailConfig.From,
		a.config.EmailConfig.To,
		[]byte(emailBody),
	)

	if err != nil {
		log.Printf("Failed to send email alert: %v", err)
		return
	}

	log.Printf("Email alert sent successfully to: %s", to)
}

//send alerts to teams 

func (a *Alerter) sendTeamsAlert(endpoint Endpoint, state *EndpointState) {

	if !a.config.TeamsEnabled || a.config.TeamsWebhook == "" {
		return
	}
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		loc = time.FixedZone("IST", 5*60*60+30*60)
	}

	istTime := state.LastCheck.In(loc)

	payload := map[string]interface{}{
		"service":        endpoint.Name,
		"url":            endpoint.URL,
		"status":         string(state.Status),
		"failures":       state.ConsecutiveFailures,
		"response_time":  state.ResponseTime.String(),
		"timestamp":      istTime.Format("02 Jan 2006, 03:04:05 PM"),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Teams alert marshal error: %v", err)
		return
	}

	resp, err := http.Post(
		a.config.TeamsWebhook,
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		log.Printf("Teams alert failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("Teams alert sent for %s", endpoint.Name)
	} else {
		log.Printf("Teams webhook returned status %d", resp.StatusCode)
	}
}

