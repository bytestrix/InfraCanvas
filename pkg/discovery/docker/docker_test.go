package docker

import (
	"testing"
)

func TestNewDiscovery(t *testing.T) {
	// Test creating a new discovery instance
	discovery, err := NewDiscovery(true)
	if err != nil {
		// Docker might not be available in test environment
		t.Skipf("Docker not available: %v", err)
	}
	defer discovery.Close()
	
	if discovery == nil {
		t.Fatal("Expected discovery instance, got nil")
	}
	
	if discovery.redactor == nil {
		t.Fatal("Expected redactor to be initialized")
	}
}

func TestIsAvailable(t *testing.T) {
	discovery, err := NewDiscovery(true)
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}
	defer discovery.Close()
	
	// IsAvailable should return true if Docker is running
	available := discovery.IsAvailable()
	if !available {
		t.Skip("Docker daemon is not running")
	}
}

func TestParseImageName(t *testing.T) {
	tests := []struct {
		name       string
		imageName  string
		wantReg    string
		wantRepo   string
		wantTag    string
	}{
		{
			name:      "simple image",
			imageName: "nginx",
			wantReg:   "docker.io",
			wantRepo:  "library/nginx",
			wantTag:   "latest",
		},
		{
			name:      "image with tag",
			imageName: "nginx:1.21",
			wantReg:   "docker.io",
			wantRepo:  "library/nginx",
			wantTag:   "1.21",
		},
		{
			name:      "user image",
			imageName: "myuser/myapp:v1.0",
			wantReg:   "docker.io",
			wantRepo:  "myuser/myapp",
			wantTag:   "v1.0",
		},
		{
			name:      "custom registry",
			imageName: "gcr.io/myproject/myapp:latest",
			wantReg:   "gcr.io",
			wantRepo:  "myproject/myapp",
			wantTag:   "latest",
		},
		{
			name:      "registry with port",
			imageName: "localhost:5000/myapp:dev",
			wantReg:   "localhost:5000",
			wantRepo:  "myapp",
			wantTag:   "dev",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotReg, gotRepo, gotTag := parseImageName(tt.imageName)
			
			if gotReg != tt.wantReg {
				t.Errorf("parseImageName() registry = %v, want %v", gotReg, tt.wantReg)
			}
			if gotRepo != tt.wantRepo {
				t.Errorf("parseImageName() repository = %v, want %v", gotRepo, tt.wantRepo)
			}
			if gotTag != tt.wantTag {
				t.Errorf("parseImageName() tag = %v, want %v", gotTag, tt.wantTag)
			}
		})
	}
}

func TestCalculateCPUPercent(t *testing.T) {
	tests := []struct {
		name           string
		cpuUsage       uint64
		prevCPUUsage   uint64
		systemUsage    uint64
		prevSystemUsage uint64
		numCPUs        uint64
		want           float64
	}{
		{
			name:            "zero delta",
			cpuUsage:        1000,
			prevCPUUsage:    1000,
			systemUsage:     10000,
			prevSystemUsage: 10000,
			numCPUs:         2,
			want:            0.0,
		},
		{
			name:            "50% usage on 2 CPUs",
			cpuUsage:        5000,
			prevCPUUsage:    0,
			systemUsage:     10000,
			prevSystemUsage: 0,
			numCPUs:         2,
			want:            100.0,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateCPUPercent(tt.cpuUsage, tt.prevCPUUsage, tt.systemUsage, tt.prevSystemUsage, tt.numCPUs)
			if got != tt.want {
				t.Errorf("calculateCPUPercent() = %v, want %v", got, tt.want)
			}
		})
	}
}
