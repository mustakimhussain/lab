package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"time"
)

const (
	UpstashRESTURL   = "https://urlofyour.upstash.io"
	UpstashRESTToken = "Replace-with-your-Upstash-REST-Bearer-Token"
)

var ipPortRegex = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}:\d+\b`)

type ServiceConfig struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

type ServiceStatus struct {
	Name       string    `json:"name"`
	Status     string    `json:"status"`     // "up" or "down"
	LastSeen   time.Time `json:"lastSeen"`   // Timestamp of last check
	LastUpTime time.Time `json:"lastUpTime"` // Last known time the service was UP
	Error      string    `json:"error,omitempty"`
}

type HeartbeatPayload struct {
	Timestamp time.Time                `json:"timestamp"`
	Services  map[string]ServiceStatus `json:"services"`
}

func sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	return ipPortRegex.ReplaceAllString(err.Error(), "[target-service]")
}

func pingAddress(address string) (bool, error) {
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return false, err
	}
	conn.Close()
	return true, nil
}

// Fetch existing state from Redis so we don't lose previous `lastUpTime` values
func fetchPreviousPayload(key string) (*HeartbeatPayload, error) {
	reqURL := fmt.Sprintf("%s/get/%s", UpstashRESTURL, key)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+UpstashRESTToken)
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstash error HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil || wrapper.Result == "" {
		return nil, nil // No previous state stored
	}

	var payload HeartbeatPayload
	if err := json.Unmarshal([]byte(wrapper.Result), &payload); err != nil {
		return nil, err
	}

	return &payload, nil
}

func pushToUpstash(key string, value HeartbeatPayload) error {
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return err
	}

	reqURL := fmt.Sprintf("%s/set/%s", UpstashRESTURL, key)
	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+UpstashRESTToken)
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upstash error HTTP %d", resp.StatusCode)
	}

	return nil
}

func runCheck(targets []ServiceConfig) {
	now := time.Now().UTC()

	// Fetch previous status payload from Redis
	prevPayload, _ := fetchPreviousPayload("home:network:health")

	payload := HeartbeatPayload{
		Timestamp: now,
		Services:  make(map[string]ServiceStatus),
	}

	// Loop through targets and preserve or update `lastUpTime`
	for _, target := range targets {
		var prevLastUp time.Time
		if prevPayload != nil && prevPayload.Services != nil {
			if prevStatus, exists := prevPayload.Services[target.ID]; exists {
				prevLastUp = prevStatus.LastUpTime
			}
		}

		status := ServiceStatus{
			Name:       target.Name,
			Status:     "down",
			LastSeen:   now,
			LastUpTime: prevLastUp, // Default to previous known up time
		}

		if ok, err := pingAddress(target.Address); ok {
			status.Status = "up"
			status.LastUpTime = now
		} else if err != nil {
			status.Error = sanitizeError(err)
		}

		payload.Services[target.ID] = status
	}

	// Push updated payload back to Redis
	if err := pushToUpstash("home:network:health", payload); err != nil {
		fmt.Printf("[%s] Failed to push to Upstash: %v\n", now.Format(time.RFC3339), err)
		return
	}

	fmt.Printf("[%s] Network ping check completed for %d services\n", now.Format(time.RFC3339), len(targets))
}

func main() {
	targets := []ServiceConfig{
		{ID: "pihole", Name: "Pi-hole", Address: "127.0.0.1:8081"},
		{ID: "adguard", Name: "AdGuard Home", Address: "127.0.0.1:8082"},
		{ID: "jenkins", Name: "Jenkins", Address: "127.0.0.1:8080"},
		{ID: "forgejo", Name: "Forgejo", Address: "192.168.0.5:3000"},
		{ID: "grafana", Name: "Grafana", Address: "127.0.0.1:3000"},
		{ID: "prometheus", Name: "Prometheus", Address: "127.0.0.1:9090"},
	}

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	fmt.Println("Starting Network Monitor Agent with LastUpTime tracking...")
	runCheck(targets)

	for range ticker.C {
		runCheck(targets)
	}
}
