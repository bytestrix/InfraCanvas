package cmd

import (
	"bytes"
	"testing"
)

func TestRootCommand(t *testing.T) {
	// Test that root command executes without error
	rootCmd.SetArgs([]string{"--help"})
	
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("root command failed: %v", err)
	}
	
	output := out.String()
	if output == "" {
		t.Error("expected help output, got empty string")
	}
}

func TestGlobalFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantErr  bool
	}{
		{
			name:    "output format flag",
			args:    []string{"--output", "json", "--help"},
			wantErr: false,
		},
		{
			name:    "scope flag",
			args:    []string{"--scope", "host,docker", "--help"},
			wantErr: false,
		},
		{
			name:    "verbose flag",
			args:    []string{"--verbose", "--help"},
			wantErr: false,
		},
		{
			name:    "quiet flag",
			args:    []string{"--quiet", "--help"},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
