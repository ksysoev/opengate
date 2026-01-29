package router

import (
	"context"

	"github.com/ksysoev/opengate/pkg/spec"
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
func withRoute(ctx context.Context, route *spec.Route) context.Context {
	return context.WithValue(ctx, keyRoute{}, route)
}

// WithRoute adds the matched route to the context (exported).
func WithRoute(ctx context.Context, route *spec.Route) context.Context {
	return withRoute(ctx, route)
}

// GetRoute retrieves the matched route from the context.
func GetRoute(ctx context.Context) *spec.Route {
	route, ok := ctx.Value(keyRoute{}).(*spec.Route)
	if !ok {
		return nil
	}

	return route
}
