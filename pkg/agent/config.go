package agent

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the agent configuration
type Config struct {
	// Backend connection
	BackendURL  string `yaml:"backend_url" json:"backend_url"`
	AuthToken   string `yaml:"auth_token" json:"auth_token"`
	TLSInsecure bool   `yaml:"tls_insecure" json:"tls_insecure"`

	// Collection intervals (in seconds)
	HostInterval       int `yaml:"host_interval" json:"host_interval"`
	DockerInterval     int `yaml:"docker_interval" json:"docker_interval"`
	KubernetesInterval int `yaml:"kubernetes_interval" json:"kubernetes_interval"`
	HeartbeatInterval  int `yaml:"heartbeat_interval" json:"heartbeat_interval"`

	// Discovery scope
	Scope []string `yaml:"scope" json:"scope"`

	// Agent identity
	AgentID   string `yaml:"agent_id" json:"agent_id"`
	AgentName string `yaml:"agent_name" json:"agent_name"`

	// Features
	EnableRedaction bool `yaml:"enable_redaction" json:"enable_redaction"`
	EnableWatchers  bool `yaml:"enable_watchers" json:"enable_watchers"`
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	hostname, _ := os.Hostname()
	return &Config{
		BackendURL:         "http://localhost:8080",
		AuthToken:          "",
		TLSInsecure:        false,
		HostInterval:       10,
		DockerInterval:     15,
		KubernetesInterval: 20,
		HeartbeatInterval:  30,
		Scope:              []string{"host", "docker", "kubernetes"},
		AgentID:            "",
		AgentName:          hostname,
		EnableRedaction:    true,
		EnableWatchers:     true,
	}
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configPath string) (*Config, error) {
	config := DefaultConfig()

	// Load from file if provided
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Override with environment variables
	if url := os.Getenv("RIX_BACKEND_URL"); url != "" {
		config.BackendURL = url
	}
	if token := os.Getenv("RIX_AUTH_TOKEN"); token != "" {
		config.AuthToken = token
	}
	if agentID := os.Getenv("RIX_AGENT_ID"); agentID != "" {
		config.AgentID = agentID
	}
	if agentName := os.Getenv("RIX_AGENT_NAME"); agentName != "" {
		config.AgentName = agentName
	}

	return config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.BackendURL == "" {
		return fmt.Errorf("backend_url is required")
	}

	if c.AuthToken == "" {
		return fmt.Errorf("auth_token is required")
	}

	if c.HostInterval < 1 {
		return fmt.Errorf("host_interval must be at least 1 second")
	}

	if c.DockerInterval < 1 {
		return fmt.Errorf("docker_interval must be at least 1 second")
	}

	if c.KubernetesInterval < 1 {
		return fmt.Errorf("kubernetes_interval must be at least 1 second")
	}

	if c.HeartbeatInterval < 1 {
		return fmt.Errorf("heartbeat_interval must be at least 1 second")
	}

	if len(c.Scope) == 0 {
		return fmt.Errorf("scope must contain at least one layer")
	}

	validScopes := map[string]bool{"host": true, "docker": true, "kubernetes": true}
	for _, scope := range c.Scope {
		if !validScopes[scope] {
			return fmt.Errorf("invalid scope: %s (must be host, docker, or kubernetes)", scope)
		}
	}

	return nil
}

// GetHostIntervalDuration returns the host collection interval as a duration
func (c *Config) GetHostIntervalDuration() time.Duration {
	return time.Duration(c.HostInterval) * time.Second
}

// GetDockerIntervalDuration returns the Docker collection interval as a duration
func (c *Config) GetDockerIntervalDuration() time.Duration {
	return time.Duration(c.DockerInterval) * time.Second
}

// GetKubernetesIntervalDuration returns the Kubernetes collection interval as a duration
func (c *Config) GetKubernetesIntervalDuration() time.Duration {
	return time.Duration(c.KubernetesInterval) * time.Second
}

// GetHeartbeatIntervalDuration returns the heartbeat interval as a duration
func (c *Config) GetHeartbeatIntervalDuration() time.Duration {
	return time.Duration(c.HeartbeatInterval) * time.Second
}
