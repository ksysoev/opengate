// Package router provides dynamic HTTP routing based on OpenAPI specifications.
package router

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ksysoev/opengate/pkg/core/route"
)

// Router handles dynamic routing based on OpenAPI specification.
type Router struct {
	routes []routeEntry
}

type routeEntry struct {
	pattern *regexp.Regexp
	method  string
	route   route.Route
	params  []string
}

// New creates a new Router instance.
func New() *Router {
	return &Router{
		routes: make([]routeEntry, 0),
	}
}

// AddRoute adds a route to the router.
func (r *Router) AddRoute(rt *route.Route) error {
	pattern, params, err := pathToRegex(rt.Path)
	if err != nil {
		return fmt.Errorf("failed to convert path to regex: %w", err)
	}

	entry := routeEntry{
		pattern: pattern,
		method:  rt.Method,
		route:   *rt,
		params:  params,
	}

	r.routes = append(r.routes, entry)

	return nil
}

// Match finds a matching route for the given request.
// Returns the route and path parameters if found, or an error if no match is found.
func (r *Router) Match(method, path string) (*route.Route, map[string]string, error) {
	for i := range r.routes {
		entry := &r.routes[i]
		if entry.method != method {
			continue
		}

		matches := entry.pattern.FindStringSubmatch(path)
		if matches == nil {
			continue
		}

		params := make(map[string]string)
		for i, name := range entry.params {
			if i+1 < len(matches) {
				params[name] = matches[i+1]
			}
		}

		return &entry.route, params, nil
	}

	return nil, nil, fmt.Errorf("no matching route found for %s %s", method, path)
}

// pathToRegex converts an OpenAPI path pattern to a regular expression.
// OpenAPI paths use {param} syntax, which we convert to named capture groups.
func pathToRegex(path string) (*regexp.Regexp, []string, error) {
	params := make([]string, 0)
	paramRegex := regexp.MustCompile(`\{([^}]+)\}`)

	// Extract parameter names
	paramMatches := paramRegex.FindAllStringSubmatch(path, -1)
	for _, match := range paramMatches {
		if len(match) > 1 {
			params = append(params, match[1])
		}
	}

	// Convert path to regex pattern
	pattern := regexp.QuoteMeta(path)
	pattern = strings.ReplaceAll(pattern, `\{`, "{")
	pattern = strings.ReplaceAll(pattern, `\}`, "}")
	pattern = paramRegex.ReplaceAllString(pattern, `([^/]+)`)
	pattern = "^" + pattern + "$"

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to compile regex pattern: %w", err)
	}

	return compiled, params, nil
}

// GetRoutes returns all registered routes.
func (r *Router) GetRoutes() []route.Route {
	routes := make([]route.Route, len(r.routes))
	for i := range r.routes {
		routes[i] = r.routes[i].route
	}

	return routes
}
