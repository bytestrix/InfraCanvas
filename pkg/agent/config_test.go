package agent

import (
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.BackendURL == "" {
		t.Error("BackendURL should not be empty")
	}

	if config.HostInterval < 1 {
		t.Error("HostInterval should be at least 1")
	}

	if len(config.Scope) == 0 {
		t.Error("Scope should not be empty")
	}

	if !config.EnableRedaction {
		t.Error("EnableRedaction should be true by default")
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				BackendURL:         "http://localhost:8080",
				AuthToken:          "test-token",
				HostInterval:       10,
				DockerInterval:     15,
				KubernetesInterval: 20,
				HeartbeatInterval:  30,
				Scope:              []string{"host", "docker"},
			},
			wantErr: false,
		},
		{
			name: "missing backend URL",
			config: &Config{
				AuthToken:          "test-token",
				HostInterval:       10,
				DockerInterval:     15,
				KubernetesInterval: 20,
				HeartbeatInterval:  30,
				Scope:              []string{"host"},
			},
			wantErr: true,
		},
		{
			name: "missing auth token",
			config: &Config{
				BackendURL:         "http://localhost:8080",
				HostInterval:       10,
				DockerInterval:     15,
				KubernetesInterval: 20,
				HeartbeatInterval:  30,
				Scope:              []string{"host"},
			},
			wantErr: true,
		},
		{
			name: "invalid interval",
			config: &Config{
				BackendURL:         "http://localhost:8080",
				AuthToken:          "test-token",
				HostInterval:       0,
				DockerInterval:     15,
				KubernetesInterval: 20,
				HeartbeatInterval:  30,
				Scope:              []string{"host"},
			},
			wantErr: true,
		},
		{
			name: "empty scope",
			config: &Config{
				BackendURL:         "http://localhost:8080",
				AuthToken:          "test-token",
				HostInterval:       10,
				DockerInterval:     15,
				KubernetesInterval: 20,
				HeartbeatInterval:  30,
				Scope:              []string{},
			},
			wantErr: true,
		},
		{
			name: "invalid scope",
			config: &Config{
				BackendURL:         "http://localhost:8080",
				AuthToken:          "test-token",
				HostInterval:       10,
				DockerInterval:     15,
				KubernetesInterval: 20,
				HeartbeatInterval:  30,
				Scope:              []string{"invalid"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigIntervalDurations(t *testing.T) {
	config := &Config{
		HostInterval:       10,
		DockerInterval:     15,
		KubernetesInterval: 20,
		HeartbeatInterval:  30,
	}

	if config.GetHostIntervalDuration() != 10*time.Second {
		t.Errorf("GetHostIntervalDuration() = %v, want %v", config.GetHostIntervalDuration(), 10*time.Second)
	}

	if config.GetDockerIntervalDuration() != 15*time.Second {
		t.Errorf("GetDockerIntervalDuration() = %v, want %v", config.GetDockerIntervalDuration(), 15*time.Second)
	}

	if config.GetKubernetesIntervalDuration() != 20*time.Second {
		t.Errorf("GetKubernetesIntervalDuration() = %v, want %v", config.GetKubernetesIntervalDuration(), 20*time.Second)
	}

	if config.GetHeartbeatIntervalDuration() != 30*time.Second {
		t.Errorf("GetHeartbeatIntervalDuration() = %v, want %v", config.GetHeartbeatIntervalDuration(), 30*time.Second)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("RIX_BACKEND_URL", "http://test:9090")
	os.Setenv("RIX_AUTH_TOKEN", "env-token")
	os.Setenv("RIX_AGENT_ID", "test-agent-id")
	os.Setenv("RIX_AGENT_NAME", "test-agent")
	defer func() {
		os.Unsetenv("RIX_BACKEND_URL")
		os.Unsetenv("RIX_AUTH_TOKEN")
		os.Unsetenv("RIX_AGENT_ID")
		os.Unsetenv("RIX_AGENT_NAME")
	}()

	config, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if config.BackendURL != "http://test:9090" {
		t.Errorf("BackendURL = %v, want %v", config.BackendURL, "http://test:9090")
	}

	if config.AuthToken != "env-token" {
		t.Errorf("AuthToken = %v, want %v", config.AuthToken, "env-token")
	}

	if config.AgentID != "test-agent-id" {
		t.Errorf("AgentID = %v, want %v", config.AgentID, "test-agent-id")
	}

	if config.AgentName != "test-agent" {
		t.Errorf("AgentName = %v, want %v", config.AgentName, "test-agent")
	}
}
