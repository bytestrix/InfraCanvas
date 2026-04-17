package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"infracanvas/internal/models"
	"infracanvas/pkg/retry"
)

// BackendClient handles communication with the backend platform
type BackendClient struct {
	config     *Config
	httpClient *http.Client
	baseURL    string
	authToken  string
}

// NewBackendClient creates a new backend client
func NewBackendClient(config *Config) *BackendClient {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.TLSInsecure,
		},
	}

	return &BackendClient{
		config:  config,
		baseURL: config.BackendURL,
		authToken: config.AuthToken,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// RegistrationRequest represents the agent registration payload
type RegistrationRequest struct {
	AgentID   string            `json:"agent_id"`
	AgentName string            `json:"agent_name"`
	Hostname  string            `json:"hostname"`
	OS        string            `json:"os"`
	Scope     []string          `json:"scope"`
	Version   string            `json:"version"`
	Metadata  map[string]string `json:"metadata"`
}

// RegistrationResponse represents the backend's registration response
type RegistrationResponse struct {
	AgentID string `json:"agent_id"`
	Message string `json:"message"`
}

// Register registers the agent with the backend with retry logic
func (c *BackendClient) Register(req *RegistrationRequest) (*RegistrationResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal registration request: %w", err)
	}

	var regResp RegistrationResponse
	
	// Use retry logic for registration
	err = retry.Do(func() error {
		resp, err := c.doRequest("POST", "/api/v1/agents/register", data)
		if err != nil {
			return fmt.Errorf("registration request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(body))
		}

		if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
			return fmt.Errorf("failed to decode registration response: %w", err)
		}

		return nil
	}, retry.NetworkConfig())

	if err != nil {
		return nil, err
	}

	return &regResp, nil
}

// SendSnapshot sends a full infrastructure snapshot to the backend with retry logic
func (c *BackendClient) SendSnapshot(snapshot *models.InfraSnapshot) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	// Use retry logic for snapshot sending
	return retry.Do(func() error {
		resp, err := c.doRequest("POST", "/api/v1/snapshots", data)
		if err != nil {
			return fmt.Errorf("snapshot request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("snapshot failed with status %d: %s", resp.StatusCode, string(body))
		}

		return nil
	}, retry.NetworkConfig())
}

// SendDelta sends incremental updates to the backend with retry logic
func (c *BackendClient) SendDelta(delta *models.Delta) error {
	data, err := json.Marshal(delta)
	if err != nil {
		return fmt.Errorf("failed to marshal delta: %w", err)
	}

	// Use retry logic for delta sending
	return retry.Do(func() error {
		resp, err := c.doRequest("POST", "/api/v1/deltas", data)
		if err != nil {
			return fmt.Errorf("delta request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("delta failed with status %d: %s", resp.StatusCode, string(body))
		}

		return nil
	}, retry.NetworkConfig())
}

// Event represents a generic event to send to the backend
type Event struct {
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Data      map[string]interface{} `json:"data"`
}

// SendEvent sends an event to the backend with retry logic
func (c *BackendClient) SendEvent(event *Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Use retry logic for event sending
	return retry.Do(func() error {
		resp, err := c.doRequest("POST", "/api/v1/events", data)
		if err != nil {
			return fmt.Errorf("event request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("event failed with status %d: %s", resp.StatusCode, string(body))
		}

		return nil
	}, retry.NetworkConfig())
}

// AgentHealth represents the agent's health status
type AgentHealth struct {
	Status           string    `json:"status"`
	Uptime           int64     `json:"uptime"`
	LastCollection   time.Time `json:"last_collection"`
	CollectionErrors int       `json:"collection_errors"`
	MemoryUsage      int64     `json:"memory_usage"`
}

// SendHeartbeat sends a heartbeat to the backend
func (c *BackendClient) SendHeartbeat(health *AgentHealth) error {
	data, err := json.Marshal(health)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat: %w", err)
	}

	resp, err := c.doRequest("POST", "/api/v1/heartbeat", data)
	if err != nil {
		return fmt.Errorf("heartbeat request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("heartbeat failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Command represents a command from the backend
type Command struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Parameters map[string]interface{} `json:"parameters"`
}

// ReceiveCommands polls for commands from the backend using long-polling
func (c *BackendClient) ReceiveCommands(ctx context.Context) (<-chan Command, error) {
	commandChan := make(chan Command, 10)

	go func() {
		defer close(commandChan)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Long-polling request with timeout
				req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/commands", nil)
				if err != nil {
					time.Sleep(5 * time.Second)
					continue
				}

				req.Header.Set("Authorization", "Bearer "+c.authToken)
				req.Header.Set("Content-Type", "application/json")

				resp, err := c.httpClient.Do(req)
				if err != nil {
					time.Sleep(5 * time.Second)
					continue
				}

				if resp.StatusCode == http.StatusOK {
					var commands []Command
					if err := json.NewDecoder(resp.Body).Decode(&commands); err == nil {
						for _, cmd := range commands {
							commandChan <- cmd
						}
					}
				}

				resp.Body.Close()

				// Short delay before next poll
				time.Sleep(2 * time.Second)
			}
		}
	}()

	return commandChan, nil
}

// doRequest performs an HTTP request with authentication
func (c *BackendClient) doRequest(method, path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.authToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}
