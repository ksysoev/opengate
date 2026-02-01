package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		expectedFields func(*testing.T, *Client)
		name           string
		cfg            Config
	}{
		{
			name: "Default values applied when config is empty",
			cfg:  Config{},
			expectedFields: func(t *testing.T, c *Client) {
				t.Helper()
				assert.NotNil(t, c.client)
				assert.Equal(t, defaultTimeout, c.client.Timeout)

				transport, ok := c.client.Transport.(*http.Transport)

				require.True(t, ok, "Transport should be *http.Transport")
				assert.Equal(t, defaultMaxIdleConns, transport.MaxIdleConns)
				assert.Equal(t, defaultMaxConnsPerHost, transport.MaxConnsPerHost)
				assert.Equal(t, defaultIdleConnTimeout, transport.IdleConnTimeout)
				assert.False(t, transport.DisableKeepAlives)
			},
		},
		{
			name: "Custom values used when specified",
			cfg: Config{
				Timeout:           5 * time.Second,
				MaxIdleConns:      50,
				MaxConnsPerHost:   5,
				IdleConnTimeout:   60 * time.Second,
				DisableKeepAlives: true,
			},
			expectedFields: func(t *testing.T, c *Client) {
				t.Helper()
				assert.NotNil(t, c.client)
				assert.Equal(t, 5*time.Second, c.client.Timeout)
				transport, ok := c.client.Transport.(*http.Transport)
				require.True(t, ok, "Transport should be *http.Transport")
				assert.Equal(t, 50, transport.MaxIdleConns)
				assert.Equal(t, 5, transport.MaxConnsPerHost)
				assert.Equal(t, 60*time.Second, transport.IdleConnTimeout)
				assert.True(t, transport.DisableKeepAlives)
			},
		},
		{
			name: "InsecureSkipVerify enables TLS config",
			cfg: Config{
				InsecureSkipVerify: true,
			},
			expectedFields: func(t *testing.T, c *Client) {
				t.Helper()

				transport, ok := c.client.Transport.(*http.Transport)

				require.True(t, ok, "Transport should be *http.Transport")
				require.NotNil(t, transport.TLSClientConfig)
				assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.cfg)
			require.NoError(t, err)
			assert.NotNil(t, client)
			tt.expectedFields(t, client)
		})
	}
}

func TestClient_Do_Success(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "test-value", r.Header.Get("X-Test-Header"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client, err := New(Config{})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/test-path", http.NoBody)
	require.NoError(t, err)
	req.Header.Set("X-Test-Header", "test-value")

	resp, err := client.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"status":"ok"}`, string(body))
}

func TestClient_Do_Timeout(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create client with short timeout
	client, err := New(Config{
		Timeout: 50 * time.Millisecond,
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, server.URL, http.NoBody)
	require.NoError(t, err)

	resp, err := client.Do(req)

	if resp != nil {
		defer resp.Body.Close()
	}

	assert.Error(t, err)
}

func TestClient_Do_RedirectNotFollowed(t *testing.T) {
	redirectCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectCount++

		if r.URL.Path == "/original" {
			w.Header().Set("Location", "/redirected")
			w.WriteHeader(http.StatusFound)

			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(Config{})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/original", http.NoBody)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	// Should return the redirect response, not follow it
	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Equal(t, "/redirected", resp.Header.Get("Location"))
	assert.Equal(t, 1, redirectCount, "Should only make one request (not follow redirect)")
}

func TestClient_Do_InvalidURL(t *testing.T) {
	client, err := New(Config{})
	require.NoError(t, err)

	// Create a request with an invalid URL that will fail during execution
	req, err := http.NewRequest(http.MethodGet, "http://invalid-host-that-does-not-exist-12345.com", http.NoBody)
	require.NoError(t, err)

	resp, err := client.Do(req)

	if resp != nil {
		defer resp.Body.Close()
	}

	assert.Error(t, err)
}

func TestNew_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		wantErr string
		cfg     Config
	}{
		{
			name: "Negative timeout",
			cfg: Config{
				Timeout: -1 * time.Second,
			},
			wantErr: "timeout must be non-negative",
		},
		{
			name: "Negative MaxIdleConns",
			cfg: Config{
				MaxIdleConns: -1,
			},
			wantErr: "max_idle_conns must be non-negative",
		},
		{
			name: "Negative MaxConnsPerHost",
			cfg: Config{
				MaxConnsPerHost: -5,
			},
			wantErr: "max_conns_per_host must be non-negative",
		},
		{
			name: "Negative IdleConnTimeout",
			cfg: Config{
				IdleConnTimeout: -10 * time.Second,
			},
			wantErr: "idle_conn_timeout must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.cfg)
			assert.Error(t, err)
			assert.Nil(t, client)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
