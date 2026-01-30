package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name         string
		configYAML   string
		envVars      map[string]string
		wantSpecPath string
		wantListen   string
		wantErr      bool
	}{
		{
			name: "Valid config file",
			configYAML: `
api:
  listen: ":8080"
gateway:
  spec_path: "/path/to/spec.json"
`,
			wantSpecPath: "/path/to/spec.json",
			wantListen:   ":8080",
			wantErr:      false,
		},
		{
			name: "Config with environment variable override",
			configYAML: `
api:
  listen: ":8080"
gateway:
  spec_path: "/path/to/spec.json"
`,
			envVars: map[string]string{
				"GATEWAY_SPEC_PATH": "/override/spec.json",
			},
			wantSpecPath: "/override/spec.json",
			wantListen:   ":8080",
			wantErr:      false,
		},
		{
			name:       "Environment variables only",
			configYAML: "",
			envVars: map[string]string{
				"API_LISTEN":        ":9090",
				"GATEWAY_SPEC_PATH": "/env/spec.json",
			},
			wantSpecPath: "/env/spec.json",
			wantListen:   ":9090",
			wantErr:      false,
		},
		{
			name: "Empty config file",
			configYAML: `
api:
  listen: ""
gateway:
  spec_path: ""
`,
			wantSpecPath: "",
			wantListen:   "",
			wantErr:      false,
		},
		{
			name: "Invalid YAML",
			configYAML: `
api:
  listen: ":8080"
  invalid yaml [[[
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file if needed
			var configPath string

			if tt.configYAML != "" {
				tmpDir := t.TempDir()
				configPath = filepath.Join(tmpDir, "config.yml")
				err := os.WriteFile(configPath, []byte(tt.configYAML), 0o600)
				require.NoError(t, err)
			}

			// Set environment variables
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			// Load config
			flags := &cmdFlags{
				ConfigPath: configPath,
			}

			cfg, err := loadConfig(flags)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantSpecPath, cfg.Gateway.SpecPath)
			assert.Equal(t, tt.wantListen, cfg.API.Listen)
		})
	}
}

func TestLoadConfig_NoConfigFile(t *testing.T) {
	// Test loading with no config file (environment variables only)
	t.Setenv("API_LISTEN", ":7070")
	t.Setenv("GATEWAY_SPEC_PATH", "/test/spec.json")

	flags := &cmdFlags{
		ConfigPath: "", // No config file
	}

	cfg, err := loadConfig(flags)

	require.NoError(t, err)
	assert.Equal(t, "/test/spec.json", cfg.Gateway.SpecPath)
	assert.Equal(t, ":7070", cfg.API.Listen)
}

func TestLoadConfig_NonExistentConfigFile(t *testing.T) {
	flags := &cmdFlags{
		ConfigPath: "/nonexistent/config.yml",
	}

	_, err := loadConfig(flags)

	assert.Error(t, err)
}
