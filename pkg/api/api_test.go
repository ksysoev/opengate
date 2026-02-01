package api

import (
	"context"
	"testing"
	"time"

	"github.com/ksysoev/opengate/pkg/core"
	"github.com/ksysoev/opengate/pkg/core/proxy"
	"github.com/ksysoev/opengate/pkg/core/redirect"
	"github.com/ksysoev/opengate/pkg/core/route"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockParser is a test implementation of specParser
type mockParser struct {
	err    error
	routes []route.Route
}

func (m *mockParser) ParseFile(filePath string) ([]route.Route, error) {
	return m.routes, m.err
}

func TestNew_ValidConfig(t *testing.T) {
	cfg := Config{Listen: ":8080"}

	// Create a core service with mock parser
	parser := &mockParser{
		routes: []route.Route{
			{
				Path:   "/test",
				Method: "GET",
				Handler: route.Handler{
					Type:    "forward",
					BaseURL: "http://backend",
				},
			},
		},
	}
	svc := core.New(parser)
	svc.RegisterHandler("forward", proxy.New())
	require.NoError(t, svc.LoadSpec(context.Background(), "test.yaml"))

	api, err := New(cfg, svc)

	assert.NoError(t, err)
	assert.NotNil(t, api)
}

func TestNew_InvalidConfig(t *testing.T) {
	cfg := Config{Listen: ""}

	parser := &mockParser{
		routes: []route.Route{
			{
				Path:   "/test",
				Method: "GET",
				Handler: route.Handler{
					Type:    "forward",
					BaseURL: "http://backend",
				},
			},
		},
	}
	svc := core.New(parser)
	svc.RegisterHandler("forward", proxy.New())
	require.NoError(t, svc.LoadSpec(context.Background(), "test.yaml"))

	_, err := New(cfg, svc)

	assert.Error(t, err)
}

func TestAPI_Run_StartAndShutdown(t *testing.T) {
	cfg := Config{Listen: "127.0.0.1:0"}

	parser := &mockParser{
		routes: []route.Route{
			{
				Path:   "/test",
				Method: "GET",
				Handler: route.Handler{
					Type:    "forward",
					BaseURL: "http://backend",
				},
			},
		},
	}
	svc := core.New(parser)
	svc.RegisterHandler("forward", proxy.New())
	svc.RegisterHandler("redirect", redirect.New())
	require.NoError(t, svc.LoadSpec(context.Background(), "test.yaml"))

	api, err := New(cfg, svc)

	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err = api.Run(ctx)

	assert.NoError(t, err)
}
