package powermux

import (
	"net/http"
	"net/url"
	"sort"
	"strings"
)

const (
	methodAny = "ANY"
	notFound  = "NOT_FOUND"
)

type childList []*Route

func (l childList) Len() int {
	return len(l)
}

func (l childList) Less(i, j int) bool {
	return l[i].pattern < l[j].pattern
}

func (l childList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l childList) Search(pattern string) *Route {
	index := sort.Search(l.Len(), func(i int) bool {
		return l[i].pattern >= pattern
	})

	if index < l.Len() && l[index].pattern == pattern {
		return l[index]
	}

	return nil
}

// routeExecution is the complete instructions for running serve on a route
type routeExecution struct {
	pattern    string
	params     map[string]string
	notFound   http.Handler
	middleware []Middleware
	handler    http.Handler
}

// A Route represents a specific path for a request.
// Routes can be absolute paths, rooted subtrees, or path parameters that accept any stringRoutes.
type Route struct {
	// the pattern our node matches
	pattern string
	// the full path to this node
	fullPath string
	// if we are a named path param node '/:name'
	isParam bool
	// the name of our path parameter
	paramName string
	// if we are a rooted sub tree '/dir/*'
	isWildcard bool
	// the array of middleware this node invokes
	middleware []Middleware
	// child nodes
	children childList
	// child node for path parameters
	paramChild *Route
	// set if there's a wildcard handler child (lowest priority)
	wildcardChild *Route
	// the map of handlers for different methods
	handlers map[string]http.Handler
}

// newRoute allocates all the structures required for a route node.
// Default pattern is "" which matches only the top level node.
func newRoute() *Route {
	return &Route{
		handlers:   make(map[string]http.Handler),
		middleware: make([]Middleware, 0),
		children:   make([]*Route, 0),
	}
}

// execute sets up the tree traversal required to get the execution instructions for
// a route.
func (r *Route) execute(method, pattern string) *routeExecution {

	pathParts := strings.Split(pattern, "/")

	if pattern == "/" {
		pathParts = pathParts[1:]
	}

	// Create a new routeExecution
	ex := &routeExecution{
		middleware: make([]Middleware, 0),
		params:     make(map[string]string),
	}

	// Fill the execution
	r.getExecution(method, pathParts, ex)

	// return the result
	return ex
}

// getExecution is a recursive step in the tree traversal. It checks to see if this node matches,
// fills out any instructions in the execution, and returns. The return value indicates only if
// this node matched, not if anything was added to the execution.
func (r *Route) getExecution(method string, pathParts []string, ex *routeExecution) {

	var curRoute *Route = r

	for {

		// save all the middleware
		ex.middleware = append(ex.middleware, curRoute.middleware...)

		// save not found handler
		if h, ok := curRoute.handlers[notFound]; ok {
			ex.notFound = h
		}

		// save options handler
		if method == http.MethodOptions {
			if h, ok := curRoute.handlers[http.MethodOptions]; ok {
				ex.handler = h
			}
		}

		// save path parameters
		if curRoute.isParam {
			value, err := url.PathUnescape(pathParts[0])
			if err != nil {
				// TODO: maybe handle errors more gracefully
				panic(err)
			}
			ex.params[curRoute.paramName] = value
		}

		// check if this is the bottom of the path
		if len(pathParts) == 1 || curRoute.isWildcard {

			// hit the bottom of the tree, see if we have a handler to offer
			curRoute.getHandler(method, ex)

			if curRoute.fullPath == "" {
				ex.pattern = "/"
			} else {
				ex.pattern = curRoute.fullPath
			}
			return

		}

		// iterate over our children looking for deeper to go

		// binary search over regular children
		if child := curRoute.children.Search(pathParts[1]); child != nil {
			pathParts = pathParts[1:]
			curRoute = child
			continue
		}

		// try for params and wildcard children
		if curRoute.paramChild != nil {
			pathParts = pathParts[1:]
			curRoute = curRoute.paramChild
			continue
		}
		if curRoute.wildcardChild != nil {
			pathParts = pathParts[1:]
			curRoute = curRoute.wildcardChild
			continue
		}

		return
	}
}

// getHandler is a convenience function for choosing a handler from the route's map of options
// Order of precedence:
// 1. An exact method match
// 2. HEAD requests can use GET handlers
// 3. The ANY handler
// 4. A generated Options handler if this is an options request and no previous handler is set
// 5. A generated Method Not Allowed response
func (r *Route) getHandler(method string, ex *routeExecution) {
	// check specific method match
	if h, ok := r.handlers[method]; ok {
		ex.handler = h
		return
	}

	// if this is a HEAD we can fall back on GET
	if method == http.MethodHead {
		if h, ok := r.handlers[http.MethodGet]; ok {
			ex.handler = h
			return
		}
	}

	// check the ANY handler
	if h, ok := r.handlers[methodAny]; ok {
		ex.handler = h
		return
	}

	// generate an options handler if none is already set
	if method == http.MethodOptions && ex.handler == nil {
		ex.handler = r.defaultOptions()
		return
	}

	// last ditch effort is to generate our own method not allowed handler
	// this is regenerated each time in case routes are added during runtime
	// not generated if a previous handler is already set
	if ex.handler == nil {
		ex.handler = r.methodNotAllowed()
	}
	return
}

// Route walks down the route tree following pattern and returns either a new or previously
// existing node that represents that specific path.
func (r *Route) Route(path string) *Route {

	// prepend a leading slash if not present
	if path[0] != '/' {
		path = "/" + path
	}

	// remove the tailing slash if it is present
	if path != "/" {
		path = strings.TrimRight(path, "/")
	}

	// append our node name to the search if we're not root
	if r.pattern != "" {
		path = r.pattern + path
	}

	// chop the path into pieces
	pathParts := strings.Split(path, "/")

	// handle the case where we're the root node
	if path == "/" {
		// strings.Split will have returned ["", ""]
		// drop one of them
		pathParts = pathParts[1:]
	}

	// find/create the new path
	return r.create(pathParts, r.fullPath)
}

// Create descends the tree following path, creating nodes as needed and returns the target node
func (r *Route) create(path []string, parentPath string) *Route {

	// ensure this path matches us
	if r.pattern != path[0] {
		// not us
		return nil
	}

	// if this is us, return, no creation necessary
	if len(path) == 1 {
		return r
	}

	// iterate over all children looking for a place to put this
	for _, child := range r.getChildren() {
		if r := child.create(path[1:], r.fullPath); r != nil {
			return r
		}
	}

	// child can't create it, so we will
	newRoute := newRoute()

	// set the pattern name
	newRoute.pattern = path[1]
	newRoute.fullPath = r.fullPath + "/" + path[1]

	// check if it's a path param
	if strings.HasPrefix(path[1], ":") {
		newRoute.isParam = true
		newRoute.paramName = strings.TrimLeft(path[1], ":")

		// save it in the correct place
		r.paramChild = newRoute

	} else if path[1] == "*" {
		// check if this is a rooted subtree
		newRoute.isWildcard = true

		// save to wildcard child
		r.wildcardChild = newRoute

		// go no deeper
		return newRoute
	} else {
		// Just a regular child
		r.children = append(r.children, newRoute)

		// sort children alphabetically for efficient run time searching
		sort.Sort(r.children)
	}

	// the cycle continues
	return newRoute.create(path[1:], r.fullPath)
}

// stringRoutes returns the stringRoutes representation of this route and all below it.
func (r *Route) stringRoutes(routes *[]string) {

	var thisRoute string

	// handle root node
	if r.fullPath == "" {
		thisRoute = "/"
	} else {
		thisRoute = r.fullPath
	}

	if len(r.handlers) > 0 {
		thisRoute = thisRoute + "\t["
		methods := make([]string, 0, 8)
		for method := range r.handlers {
			methods = append(methods, method)
		}
		thisRoute = thisRoute + strings.Join(methods, ", ") + "]"
		*routes = append(*routes, thisRoute)
	}

	// recursion
	for _, child := range r.getChildren() {
		child.stringRoutes(routes)
	}
}

// getChildren returns the all the route handler with the correct order of precedence
func (r *Route) getChildren() []*Route {

	// allocate once
	allRoutes := make([]*Route, 0, len(r.children)+2)

	// start with the normal routes
	allRoutes = append(allRoutes, r.children...)

	// then add the param child
	if r.paramChild != nil {
		allRoutes = append(allRoutes, r.paramChild)
	}

	// then add the wildcard child
	if r.wildcardChild != nil {
		allRoutes = append(allRoutes, r.wildcardChild)
	}

	return allRoutes
}

// Middleware adds a middleware to this Route.
//
// Middlewares are executed if the path to the target route crosses this route.
func (r *Route) Middleware(m Middleware) *Route {
	r.middleware = append(r.middleware, m)
	return r
}

// MiddlewareFunc registers a plain function as a middleware.
func (r *Route) MiddlewareFunc(m MiddlewareFunc) *Route {
	return r.Middleware(MiddlewareFunc(m))
}

// Any registers a catch-all handler for any method sent to this route.
// This takes lower precedence than a specific method match.
func (r *Route) Any(handler http.Handler) *Route {
	r.handlers[methodAny] = handler
	return r
}

// Post adds a handler for POST methods to this route.
func (r *Route) Post(handler http.Handler) *Route {
	r.handlers[http.MethodPost] = handler
	return r
}

// Patch adds a handler for PATCH methods to this route.
func (r *Route) Patch(handler http.Handler) *Route {
	r.handlers[http.MethodPatch] = handler
	return r
}

// Get adds a handler for GET methods to this route.
// Get handlers will also be called for HEAD requests if no specific
// HEAD handler is registered.
func (r *Route) Get(handler http.Handler) *Route {
	r.handlers[http.MethodGet] = handler
	return r
}

// Delete adds a handler for DELETE methods to this route.
func (r *Route) Delete(handler http.Handler) *Route {
	r.handlers[http.MethodDelete] = handler
	return r
}

// Head adds a handler for HEAD methods to this route.
func (r *Route) Head(handler http.Handler) *Route {
	r.handlers[http.MethodHead] = handler
	return r
}

// Connect adds a handler for CONNECT methods to this route.
func (r *Route) Connect(handler http.Handler) *Route {
	r.handlers[http.MethodConnect] = handler
	return r
}

// Options adds a handler for OPTIONS methods to this route.
// This handler will also be called for any routes further down the path from
// this point if no other OPTIONS handlers are registered below.
func (r *Route) Options(handler http.Handler) *Route {
	r.handlers[http.MethodOptions] = handler
	return r
}

// NotFound adds a handler for requests that do not correspond to a route.
// This handler will also be called for any routes further down the path from
// this point if no other not found handlers are registered below.
func (r *Route) NotFound(handler http.Handler) *Route {
	r.handlers[notFound] = handler
	return r
}
