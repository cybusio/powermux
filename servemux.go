package powermux

import (
	"bytes"
	"context"
	"net/http"
	"strings"
)

// ServeMux is the multiplexer for http requests
type ServeMux struct {
	baseRoute     *Route
	hostRoutes    map[string]*Route
	executionPool *executionPool
}

// ctxKey is the key type used for path parameters in the request context
type ctxKey string

var (
	routeKey = ctxKey("route_name")
	paramKey = ctxKey("params")
)

// PathParam gets named path parameters and their values from the request
//
// the path '/users/:name' given '/users/andrew' will have `PathParam(r, "name")` => `"andrew"`
// unset values return an empty stringRoutes
func PathParam(req *http.Request, name string) (value string) {
	value = req.Context().Value(paramKey).(map[string]string)[name]
	return
}

// PathParams returns the map of all path parameters and their values from the request.
//
// Altering the values of this map will not affect future calls to PathParam and PathParams.
func PathParams(req *http.Request) (params map[string]string) {
	params = make(map[string]string)
	for k, v := range req.Context().Value(paramKey).(map[string]string) {
		params[k] = v
	}
	return
}

// RequestPath returns the path definition that the router used to serve this request,
// without any parameter substitution.
func RequestPath(req *http.Request) (value string) {
	value, _ = req.Context().Value(routeKey).(string)
	return value
}

// NewServeMux creates a new multiplexer, and sets up a default not found handler
func NewServeMux() *ServeMux {
	s := &ServeMux{
		baseRoute:     newRoute(),
		hostRoutes:    make(map[string]*Route),
		executionPool: newExecutionPool(),
	}
	s.NotFound(http.NotFoundHandler())
	return s
}

func (s *ServeMux) getAll(r *http.Request) (http.Handler, []Middleware, string, map[string]string) {
	path := r.URL.EscapedPath()

	// Check for redirect
	if path != "/" && strings.HasSuffix(path, "/") {
		r.URL.Path = strings.TrimRight(path, "/")
		redirect := http.RedirectHandler(r.URL.RequestURI(), http.StatusPermanentRedirect)
		return redirect, make([]Middleware, 0), r.URL.EscapedPath(), nil
	}

	// Get a route execution from the pool
	ex := s.executionPool.Get()
	defer s.executionPool.Put(ex)

	// fill it
	if route, ok := s.hostRoutes[r.URL.Host]; ok {
		route.execute(ex, r.Method, r.URL.EscapedPath())
	} else {
		s.baseRoute.execute(ex, r.Method, r.URL.EscapedPath())
	}

	// fall back on not found handler if necessary
	if ex.handler == nil {
		ex.handler = ex.notFound
	}

	return ex.handler, ex.middleware, ex.pattern, ex.params
}

// ServeHTTP dispatches the request to the handler whose pattern most closely matches the request URL.
func (s *ServeMux) ServeHTTP(rw http.ResponseWriter, req *http.Request) {

	handler, middleware, pattern, params := s.getAll(req)

	// Save the route path
	ctx := context.WithValue(req.Context(), routeKey, pattern)

	// set all the path params
	ctx = context.WithValue(ctx, paramKey, params)

	// Save context into request
	req = req.WithContext(ctx)

	// Run a middleware/handler closure to nest all middleware
	f := getNextMiddleware(middleware, handler)
	f(rw, req)
}

// Handle registers the handler for the given pattern.
// If a handler already exists for pattern it is overwritten.
func (s *ServeMux) Handle(path string, handler http.Handler) {
	s.Route(path).Any(handler)
}

// HandleHost registers the handler for the given pattern and host.
// If a handler already exists for pattern it is overwritten.
func (s *ServeMux) HandleHost(host, path string, handler http.Handler) {
	s.RouteHost(host, path).Any(handler)
}

// Middleware adds middleware for the given pattern.
func (s *ServeMux) Middleware(path string, middleware Middleware) {
	s.Route(path).Middleware(middleware)
}

// MiddlewareFunc registers a plain function as a middleware.
func (s *ServeMux) MiddlewareFunc(path string, m MiddlewareFunc) *Route {
	return s.Route(path).MiddlewareFunc(m)
}

// MiddlewareHost adds middleware for the given pattern.
func (s *ServeMux) MiddlewareHost(host, path string, middleware Middleware) {
	s.RouteHost(host, path).Middleware(middleware)
}

// HandleFunc registers the handler function for the given pattern.
func (s *ServeMux) HandleFunc(path string, handler func(http.ResponseWriter, *http.Request)) {
	s.Handle(path, http.HandlerFunc(handler))
}

// Handler returns the handler to use for the given request, consulting r.Method, r.Host, and r.URL.Path.
// It always returns a non-nil handler. If the path is not in its canonical form, the handler will be an
// internally-generated handler that redirects to the canonical path.
//
// Handler also returns the registered pattern that matches the request or, in the case of internally-generated
// redirects, the pattern that will match after following the redirect.
//
// If there is no registered handler that applies to the request, Handler returns a “page not found” handler
// and an empty pattern.
func (s *ServeMux) Handler(r *http.Request) (http.Handler, string) {
	handler, _, pattern := s.HandlerAndMiddleware(r)
	return handler, pattern
}

// HandlerAndMiddleware returns the same as Handler, but with the addition of an array of middleware, in the order
// they would have been executed
func (s *ServeMux) HandlerAndMiddleware(r *http.Request) (http.Handler, []Middleware, string) {
	handler, middlewares, pattern, _ := s.getAll(r)
	return handler, middlewares, pattern
}

// Route returns the route from the root of the domain to the given pattern
func (s *ServeMux) Route(path string) *Route {
	return s.baseRoute.Route(path)
}

// RouteHost returns the route from the root of the domain to the given pattern on a specific domain
func (s *ServeMux) RouteHost(host, path string) *Route {
	r, ok := s.hostRoutes[host]
	if !ok {
		r = newRoute()
		s.hostRoutes[host] = r
	}
	return r.Route(path)
}

// NotFound sets the default not found handler for the server
func (s *ServeMux) NotFound(handler http.Handler) {
	s.baseRoute.NotFound(handler)
}

// String returns a list of all routes registered with this server
func (s *ServeMux) String() string {
	routes := make([]string, 0, 1)
	s.baseRoute.stringRoutes(&routes)

	buf := bytes.Buffer{}

	for _, route := range routes {
		buf.WriteString(route + "\n")
	}

	for host, baseRoute := range s.hostRoutes {
		routes = routes[0:0]
		baseRoute.stringRoutes(&routes)
		for _, route := range routes {
			buf.WriteString(host + route + "\n")
		}
	}

	return buf.String()
}
