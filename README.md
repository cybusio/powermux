# PowerMux
A drop-in replacement for Go's `http.ServeMux` with all the missing features

PowerMux stores routes in Radix trees for fast route matching and lookup on large numbers of routes.

## Setting up PowerMux
### Using http.ServeMux syntax

You can use PowerMux exactly as you would use Go's server mux.
```go
// Golang default
mux := http.NewServeMux()
mux.Handle("/", myHandler)
  
// PowerMux
mux := powermux.NewServeMux()
mux.Handle("/", myHandler)
```

### Using the Route syntax
PowerMux also has a cleaner way to declare routes, using the `Route` function.

Each call to `Route()` returns a pointer to that particular path on the radix tree, creating it if necessary.
At each route, you can add middleware, set handlers, or descend further into the route
```go
mux := powermux.NewServeMux()
 
// Set a GET handler for "/"
mux.Route("/").Get(myHandler)
 
// Set POST/DELETE handlers for "/"
mux.Route("/").
    Post(myPostHandler).
    Delete(myDeleteHandler)
```

Sequential calls to route have the same effect as a single call with a longer path
```go
mux.Route("/a").Route("/b").Route("/c") == mux.Route("/a/b/c")
```

Since Handler methods also return the route, the syntax can also be chained like so
```go
mux.Route("/").
    Get(rootHandler).
    
    Route("/a").
    Get(aGetHandler).
    Post(aPostHandler).
    
    Route("/b").
    Get(abGetHandler)
```
## Middleware
Powermux has support for any kind of middleware that uses the common `func(res, req, next)` syntax.  
Middleware handler objects must implement the `ServeHTTPMiddleware` interface.

Middleware can be added to any Route
```go
mux.Route("/users").
    Middleware(authMiddleware).
    Get(sensitiveInfoHandler)
    
// or
mux.Route("/books").MiddleWare(loggingMiddleware)
mux.Route("/books").Get(booksHandler)
```

Middleware will be run if it's attached to any part of the route above and including the final path
```go
mux.Route("/").Middleware(midRoot)
mux.Route("/a").Middleware(midA)
mux.Route("/a/b").Middleware(midB)
mux.Route("/c").Middleware(midC)
 
// requests to /a/b will run midRoot, midA, midB, 
// then any handlers on Route("/a/b")
```

## Not Found and OPTIONS handlers
`Options` and `NotFound` handlers are treated specially. If one is not found on the Route node requested, 
the latest one above that node will be used. This allows whole sections of routes to be covered under custom CORS
responses or Not Found handlers

## Path Parameters
Routes may include path parameters, specified with `/:name`  
```go
mux.Route("/users/:id/info").
    Get(userInfoHander)
```
This will make the variable `id` available to the get handler and any middleware.  
To retrieve path parameters, use `GetPathParam()`
```go
// called with /users/andrew/info
func ServeHTTP(w http.ResponseWriter, r *http.Request) {
        id := powermux.GetPathParam(r, "id")
        // id == "andrew"
}
```

Path parameters that aren't found return an empty string