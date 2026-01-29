package router

import (
	"context"

	"github.com/ksysoev/opengate/pkg/core/route"
)

type keyPathParam struct{}
type keyRoute struct{}

// withPathParam adds a path parameter to the context.
func withPathParam(ctx context.Context, key, value string) context.Context {
	params, ok := ctx.Value(keyPathParam{}).(map[string]string)
	if !ok {
		params = make(map[string]string)
	}

	params[key] = value

	return context.WithValue(ctx, keyPathParam{}, params)
}

// WithPathParam adds a path parameter to the context (exported).
func WithPathParam(ctx context.Context, key, value string) context.Context {
	return withPathParam(ctx, key, value)
}

// GetPathParams retrieves all path parameters from the context.
func GetPathParams(ctx context.Context) map[string]string {
	params, ok := ctx.Value(keyPathParam{}).(map[string]string)
	if !ok {
		return make(map[string]string)
	}

	return params
}

// GetPathParam retrieves a specific path parameter from the context.
func GetPathParam(ctx context.Context, key string) string {
	params := GetPathParams(ctx)
	return params[key]
}

// withRoute adds the matched route to the context.
func withRoute(ctx context.Context, rt *route.Route) context.Context {
	return context.WithValue(ctx, keyRoute{}, rt)
}

// WithRoute adds the matched route to the context (exported).
func WithRoute(ctx context.Context, rt *route.Route) context.Context {
	return withRoute(ctx, rt)
}

// GetRoute retrieves the matched route from the context.
func GetRoute(ctx context.Context) *route.Route {
	rt, ok := ctx.Value(keyRoute{}).(*route.Route)
	if !ok {
		return nil
	}

	return rt
}
