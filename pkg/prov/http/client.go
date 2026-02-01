// Package http provides HTTP client functionality for making requests to backend services.
package http

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"
)

const (
	defaultTimeout         = 30 * time.Second
	defaultMaxIdleConns    = 100
	defaultMaxConnsPerHost = 10
	defaultIdleConnTimeout = 90 * time.Second
)

// Config holds the configuration for the HTTP client provider.
type Config struct {
	Timeout            time.Duration `mapstructure:"timeout"`
	MaxIdleConns       int           `mapstructure:"max_idle_conns"`
	MaxConnsPerHost    int           `mapstructure:"max_conns_per_host"`
	IdleConnTimeout    time.Duration `mapstructure:"idle_conn_timeout"`
	DisableKeepAlives  bool          `mapstructure:"disable_keep_alives"`
	InsecureSkipVerify bool          `mapstructure:"insecure_skip_verify"`
}

// Client wraps http.Client and provides HTTP request functionality.
type Client struct {
	client *http.Client
}

// New creates a new HTTP client with the provided configuration.
// It applies sensible defaults for production use if values are not specified.
func New(cfg Config) *Client {
	// Apply defaults
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultTimeout
	}

	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = defaultMaxIdleConns
	}

	if cfg.MaxConnsPerHost == 0 {
		cfg.MaxConnsPerHost = defaultMaxConnsPerHost
	}

	if cfg.IdleConnTimeout == 0 {
		cfg.IdleConnTimeout = defaultIdleConnTimeout
	}

	// Create custom transport with connection pooling settings
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxConnsPerHost:       cfg.MaxConnsPerHost,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     cfg.DisableKeepAlives,
	}

	// Configure TLS if insecure skip verify is enabled
	if cfg.InsecureSkipVerify {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // Configurable for development/testing
		}
	}

	return &Client{
		client: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Don't follow redirects - let the gateway pass them through
				return http.ErrUseLastResponse
			},
		},
	}
}

// Do executes an HTTP request and returns the response.
// The caller is responsible for closing the response body.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	return resp, nil
}
