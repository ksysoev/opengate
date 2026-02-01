package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ksysoev/opengate/pkg/core/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCommand_MissingSpecPath(t *testing.T) {
	t.Cleanup(middleware.ResetRegistryForTest)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	configYAML := `
api:
  listen: ":0"
gateway:
  spec_path: ""
`
	err := os.WriteFile(configPath, []byte(configYAML), 0o600)
	require.NoError(t, err)

	flags := &cmdFlags{
		ConfigPath: configPath,
		LogLevel:   "info",
		TextFormat: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = RunCommand(ctx, flags)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gateway spec path must be specified")
}

func TestRunCommand_InvalidSpecPath(t *testing.T) {
	t.Cleanup(middleware.ResetRegistryForTest)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	configYAML := `
api:
  listen: ":0"
gateway:
  spec_path: "/nonexistent/spec.json"
`
	err := os.WriteFile(configPath, []byte(configYAML), 0o600)
	require.NoError(t, err)

	flags := &cmdFlags{
		ConfigPath: configPath,
		LogLevel:   "info",
		TextFormat: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = RunCommand(ctx, flags)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load OpenAPI spec")
}

func TestRunCommand_InvalidSpecJSON(t *testing.T) {
	t.Cleanup(middleware.ResetRegistryForTest)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	specPath := filepath.Join(tmpDir, "spec.json")

	// Write invalid JSON
	err := os.WriteFile(specPath, []byte("invalid json {{{"), 0o600)
	require.NoError(t, err)

	configYAML := `
api:
  listen: ":0"
gateway:
  spec_path: "` + specPath + `"
`
	err = os.WriteFile(configPath, []byte(configYAML), 0o600)
	require.NoError(t, err)

	flags := &cmdFlags{
		ConfigPath: configPath,
		LogLevel:   "info",
		TextFormat: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = RunCommand(ctx, flags)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load OpenAPI spec")
}

func TestRunCommand_SuccessfulStartup(t *testing.T) {
	t.Cleanup(middleware.ResetRegistryForTest)

	// Create a mock backend server
	backendCalled := false

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendCalled = true

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "success"}`))
	}))
	defer backend.Close()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	specPath := filepath.Join(tmpDir, "spec.json")

	// Create OpenAPI spec pointing to our test backend
	spec := map[string]interface{}{
		"openapi": "3.1.0",
		"info": map[string]string{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/api/test": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "test-get",
					"x-opengate": map[string]interface{}{
						"type": "forward",
						"options": map[string]string{
							"url": backend.URL,
						},
					},
				},
			},
		},
	}

	specJSON, err := json.Marshal(spec)
	require.NoError(t, err)
	err = os.WriteFile(specPath, specJSON, 0o600)
	require.NoError(t, err)

	// Create config
	configYAML := `
api:
  listen: ":0"
gateway:
  spec_path: "` + specPath + `"
`
	err = os.WriteFile(configPath, []byte(configYAML), 0o600)
	require.NoError(t, err)

	flags := &cmdFlags{
		ConfigPath: configPath,
		LogLevel:   "error", // Reduce log noise
		TextFormat: true,
	}

	// Run server in background
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errChan := make(chan error, 1)

	go func() {
		errChan <- RunCommand(ctx, flags)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Note: We can't easily test the actual HTTP request here because
	// the server binds to ":0" which picks a random port, and we don't
	// have a way to get that port back from RunCommand.
	// This test verifies that RunCommand can successfully parse config,
	// load spec, and start the server without errors during initialization.

	// Wait for context cancellation or error
	select {
	case err := <-errChan:
		// Server stopped - could be due to context cancellation (expected) or error
		if err != nil && ctx.Err() == nil {
			t.Fatalf("Server returned unexpected error: %v", err)
		}
	case <-ctx.Done():
		// Expected - context timeout
	}

	// Verify backend was NOT called (we couldn't make requests)
	assert.False(t, backendCalled, "Backend should not have been called")
}

func TestRunCommand_EmptySpec(t *testing.T) {
	t.Cleanup(middleware.ResetRegistryForTest)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	specPath := filepath.Join(tmpDir, "spec.json")

	// Create empty but valid OpenAPI spec
	spec := map[string]interface{}{
		"openapi": "3.1.0",
		"info": map[string]string{
			"title":   "Empty API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{},
	}

	specJSON, err := json.Marshal(spec)
	require.NoError(t, err)
	err = os.WriteFile(specPath, specJSON, 0o600)
	require.NoError(t, err)

	configYAML := `
api:
  listen: ":0"
gateway:
  spec_path: "` + specPath + `"
`
	err = os.WriteFile(configPath, []byte(configYAML), 0o600)
	require.NoError(t, err)

	flags := &cmdFlags{
		ConfigPath: configPath,
		LogLevel:   "error",
		TextFormat: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = RunCommand(ctx, flags)

	// Empty spec should fail with "no routes found" error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no routes found")
}
