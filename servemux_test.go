package powermux

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type dummyHandler string

func (h dummyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, string(h))
}

func (h dummyHandler) ServeHTTPMiddleware(w http.ResponseWriter, r *http.Request, n NextMiddlewareFunc) {
	io.WriteString(w, string(h))
	n(w, r)
}

func dummyHandlerFunc(response string) func (w http.ResponseWriter, r *http.Request) {
	return func (w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, response)
	}
}

var (
	rightHandler = dummyHandler("right")
	wrongHandler = dummyHandler("wrong")
	mid1         = dummyHandler("mid1")
	mid2         = dummyHandler("mid2")
)

// Ensures that parameter routes have lower precedence than absolute routes
func TestServeMux_ParamPrecedence(t *testing.T) {
	s := NewServeMux()

	s.Route("/users/:id/info").Get(wrongHandler)
	s.Route("/users/jim/info").Get(rightHandler)
	s.Route("/users/:id/detail").Get(wrongHandler)

	req := httptest.NewRequest(http.MethodGet, "/users/jim/info", nil)
	h, path := s.Handler(req)

	if h != rightHandler {
		t.Error("Wrong handler returned")
	}

	if path != "/users/jim/info" {
		t.Errorf("Wrong string path: %s", path)
	}
}

// Ensures that parameter routes have lower precedence than absolute routes
// and path parameter is properly extracted
func TestServeMux_ParamPrecedenceParamExtraction(t *testing.T) {
	s := NewServeMux()

	var called bool
	var param string

	rightHandler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		called = true
		param = PathParam(req, "id")
	})

	s.Route("/users/:id/info").Get(wrongHandler)
	s.Route("/users/jim/info").Get(rightHandler)
	s.Route("/users/:id/detail").Get(wrongHandler)

	req := httptest.NewRequest(http.MethodGet, "/users/jim/info", nil)
	s.ServeHTTP(nil, req)

	if !called {
		t.Error("None or wrong handler was called")
	}

	if param != "" {
		t.Error("Wrong path param returned")
	}
}

// Ensures that wildcards have the lowest of all precedences
func TestServeMux_WildcardPrecedence(t *testing.T) {
	s := NewServeMux()

	s.Route("/users/*").Get(wrongHandler)
	s.Route("/users/john").Get(rightHandler)

	req := httptest.NewRequest(http.MethodGet, "/users/john", nil)
	h, path := s.Handler(req)

	if h != rightHandler {
		t.Error("Wrong handler returned")
	}

	if path != "/users/john" {
		t.Errorf("Wrong string path: %s", path)
	}
}

// Ensures the wildcard handler isn't called when a path param was available
func TestServeMux_WildcardPathPrecedence(t *testing.T) {
	s := NewServeMux()

	s.Route("/users/*").Get(wrongHandler)
	s.Route("/users/:id").Get(rightHandler)

	req := httptest.NewRequest(http.MethodGet, "/users/john", nil)
	h, path := s.Handler(req)

	if h != rightHandler {
		t.Error("Wrong handler returned")
	}

	if path != "/users/:id" {
		t.Errorf("Wrong string path: %s", path)
	}
}

// Ensures the wildcard handler isn't called when a path param was available
// and path parameter is properly extracted
func TestServeMux_WildcardPathPrecedenceParamExtraction(t *testing.T) {
	s := NewServeMux()

	var called bool
	var param string

	rightHandler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		called = true
		param = PathParam(req, "id")
	})

	s.Route("/users/*").Get(wrongHandler)
	s.Route("/users/:id").Get(rightHandler)

	req := httptest.NewRequest(http.MethodGet, "/users/john", nil)
	s.ServeHTTP(nil, req)

	if !called {
		t.Error("None or wrong handler was called")
	}

	if param != "john" {
		t.Error("Wrong path param returned")
	}
}

// Ensures trailing slash redirects are working
func TestServeMux_RedirectSlash(t *testing.T) {
	s := NewServeMux()

	req := httptest.NewRequest(http.MethodGet, "/users/", nil)
	rec := httptest.NewRecorder()

	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusPermanentRedirect {
		t.Error("Not redirected")
	}

	if rec.HeaderMap.Get("Location") != "/users" {
		t.Error("Mis-redirected")
	}
}

// Ensures we don't redirect the root
func TestServeMux_RedirectRoot(t *testing.T) {
	s := NewServeMux()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	s.ServeHTTP(rec, req)

	if rec.Code == http.StatusPermanentRedirect {
		t.Error("Redirected")
	}
}

// Ensure the correct path is matched 1 level
func TestServeMux_HandleCorrectRoute(t *testing.T) {
	s := NewServeMux()

	s.Route("/a").Get(rightHandler)
	s.Route("/b").Get(wrongHandler)

	req := httptest.NewRequest(http.MethodGet, "/a", nil)
	h, path := s.Handler(req)

	if h != rightHandler {
		t.Error("Wrong handler returned")
	}

	if path != "/a" {
		t.Errorf("Wrong string path: %s", path)
	}
}

// Ensure the correct path is matched at two levels
func TestServeMux_HandleCorrectRouteAfterParam(t *testing.T) {
	s := NewServeMux()

	s.Route("/base/:id/a").Get(rightHandler)
	s.Route("/base/:id/b").Get(wrongHandler)

	req := httptest.NewRequest(http.MethodGet, "/base/llama/a", nil)
	h, path := s.Handler(req)

	if h != rightHandler {
		t.Error("Wrong handler retured")
	}

	if path != "/base/:id/a" {
		t.Errorf("Wrong string path: %s", path)
	}
}

// Ensure the correct path is matched at two levels
// and path parameter is properly extracted
func TestServeMux_HandleCorrectRouteAfterParamExtraction(t *testing.T) {
	s := NewServeMux()

	var called bool
	var param string

	rightHandler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		called = true
		param = PathParam(req, "id")
	})

	s.Route("/base/:id/a").Get(rightHandler)
	s.Route("/base/:id/b").Get(wrongHandler)

	req := httptest.NewRequest(http.MethodGet, "/base/llama/a", nil)
	s.ServeHTTP(nil, req)

	if !called {
		t.Error("None or wrong handler was called")
	}

	if param != "llama" {
		t.Error("Wrong path param returned")
	}
}

// Ensure the correct method is matched
func TestServeMux_HandleCorrectMethod(t *testing.T) {
	s := NewServeMux()

	s.Route("/a").Post(rightHandler)
	s.Route("/a").Get(wrongHandler)

	req := httptest.NewRequest(http.MethodPost, "/a", nil)
	h, path := s.Handler(req)

	if h != rightHandler {
		t.Error("Wrong handler returned")
	}

	if path != "/a" {
		t.Errorf("Wrong string path: %s", path)
	}
}

// Ensure the correct method is matched for any
func TestServeMux_HandleCorrectMethodAny(t *testing.T) {
	s := NewServeMux()

	s.Route("/a").Post(wrongHandler)
	s.Route("/a").Get(wrongHandler)
	s.Route("/a").Any(rightHandler)

	req := httptest.NewRequest(http.MethodDelete, "/a", nil)
	h, path := s.Handler(req)

	if h != rightHandler {
		t.Error("Wrong handler returned")
	}

	if path != "/a" {
		t.Errorf("Wrong string path: %s", path)
	}
}

// Ensure the correct method is matched for head
func TestServeMux_HandleCorrectMethodHead(t *testing.T) {
	s := NewServeMux()

	s.Route("/a").Post(wrongHandler)
	s.Route("/a").Get(rightHandler)

	req := httptest.NewRequest(http.MethodHead, "/a", nil)
	h, path := s.Handler(req)

	if h != rightHandler {
		t.Error("Wrong handler returned")
	}

	if path != "/a" {
		t.Errorf("Wrong string path: %s", path)
	}
}

// Ensure a wildcard matches
func TestServeMux_HandleWildcard(t *testing.T) {
	s := NewServeMux()

	s.Route("/a/*").Get(rightHandler)
	s.Route("/b").Get(wrongHandler)

	req := httptest.NewRequest(http.MethodGet, "/a/llama", nil)
	h, path := s.Handler(req)

	if h != rightHandler {
		t.Error("Wrong handler returned")
	}

	if path != "/a/*" {
		t.Errorf("Wrong string path: %s", path)
	}
}

// Ensure a wildcard matches at depth
func TestServeMux_HandleWildcardDepth(t *testing.T) {
	s := NewServeMux()

	s.Route("/a/*").Get(rightHandler)
	s.Route("/b").Get(wrongHandler)

	req := httptest.NewRequest(http.MethodGet, "/a/llama/4/5", nil)
	h, path := s.Handler(req)

	if h != rightHandler {
		t.Error("Wrong handler returned")
	}

	if path != "/a/*" {
		t.Errorf("Wrong string path: %s", path)
	}
}

// Ensure order doesn't matter
func TestServeMux_HandleOrder(t *testing.T) {
	s := NewServeMux()

	s.Route("/a").Get(wrongHandler)
	s.Route("/b").Get(rightHandler)

	req := httptest.NewRequest(http.MethodGet, "/b", nil)
	h, path := s.Handler(req)

	if h != rightHandler {
		t.Error("Wrong handler returned")
	}

	if path != "/b" {
		t.Errorf("Wrong string path: %s", path)
	}
}

func TestServeMux_HandleOptionsAtDepth(t *testing.T) {
	s := NewServeMux()

	s.Route("/a").Options(rightHandler)
	s.Route("/a/b").Get(wrongHandler)

	req := httptest.NewRequest(http.MethodOptions, "/a/b", nil)
	h, path := s.Handler(req)

	if h != rightHandler {
		t.Error("Wrong handler returned")
	}

	if path != "/a/b" {
		t.Errorf("Wrong string path: %s", path)
	}
}

// Ensure routing is not performed on decoded path components
func TestServeMux_EncodedPathComponent(t *testing.T) {
	s := NewServeMux()

	s.Route("/users/:id/info").Get(rightHandler)

	req := httptest.NewRequest(http.MethodGet, "/users/ji%2Fm/info", nil)
	h, path := s.Handler(req)

	if h != rightHandler {
		t.Error("Wrong handler returned")
	}

	if path != "/users/:id/info" {
		t.Errorf("Wrong string path: %s", path)
	}
}

// Ensure routing is not performed on decoded path components
// and path parameter is properly extracted
func TestServeMux_EncodedPathComponentParamExtraction(t *testing.T) {
	s := NewServeMux()

	var called bool
	var param string

	rightHandler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		called = true
		param = PathParam(req, "id")
	})

	s.Route("/users/:id/info").Get(rightHandler)

	req := httptest.NewRequest(http.MethodGet, "/users/ji%2Fm/info", nil)
	s.ServeHTTP(nil, req)

	if !called {
		t.Error("None or wrong handler was called")
	}

	if param != "ji/m" {
		t.Error("Wrong path param returned")
	}
}

func TestRoute_PermanentRedirect(t *testing.T) {
	s := NewServeMux()

	s.Route("/redir").Redirect("/redirect", true)

	req := httptest.NewRequest(http.MethodGet, "/redir", nil)
	res := httptest.NewRecorder()

	s.ServeHTTP(res, req)

	if res.Code != http.StatusPermanentRedirect {
		t.Error("Should have issued permanemt redirect. Got", res.Code)
	}

	if res.Header().Get("Location") != "/redirect" {
		t.Error("Wrong redirect target. Expected /redirect, got", res.Header().Get("Location"))
	}

}

func TestRoute_TemporaryRedirect(t *testing.T) {
	s := NewServeMux()

	s.Route("/redir").Redirect("/redirect", false)

	req := httptest.NewRequest(http.MethodGet, "/redir", nil)
	res := httptest.NewRecorder()

	s.ServeHTTP(res, req)

	if res.Code != http.StatusTemporaryRedirect {
		t.Error("Should have issued temporary redirect. Got", res.Code)
	}

	if res.Header().Get("Location") != "/redirect" {
		t.Error("Wrong redirect target. Expected /redirect, got", res.Header().Get("Location"))
	}

}

func TestNotFoundEmptyRouteNode(t *testing.T) {
	s := NewServeMux()

	// create but add no handlers
	s.Route("/empty")

	req := httptest.NewRequest(http.MethodGet, "/empty", nil)
	res := httptest.NewRecorder()

	s.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Error("Wrong response code, expected not found, got", res.Code)
	}
}

func TestRoute_Head(t *testing.T) {

	s := NewServeMux()

	s.Route("/").Get(rightHandler)

	req := httptest.NewRequest(http.MethodHead, "/", nil)

	h, path := s.Handler(req)

	if h != rightHandler {
		t.Error("Wrong handler returned")
	}

	if path != "/" {
		t.Error("Wrong path returned", path)
	}

}

func TestRoutePathRoot(t *testing.T) {
	s := NewServeMux()

	s.Route("/").Get(rightHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	h, path := s.Handler(req)

	if h != rightHandler {
		t.Error("Wrong handler returned")
	}

	if path != "/" {
		t.Error("Wrong path returned", path)
	}
}

func TestNotFoundFallback(t *testing.T) {
	s := NewServeMux()

	req := httptest.NewRequest(http.MethodGet, "/found", nil)
	res := httptest.NewRecorder()

	s.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Error("Wrong response code. Expected 404 got", res.Code)
	}
}

func TestServeMux_HandleGet(t *testing.T) {
	s := NewServeMux()

	s.Handle("/a", rightHandler)
	req := httptest.NewRequest(http.MethodGet, "/a", nil)

	h, path := s.Handler(req)

	if h != rightHandler {
		t.Error("Wrong handler returned")
	}

	if path != "/a" {
		t.Error("Wrong path, expected /a, got", path)
	}
}

func TestServeMux_HandlePost(t *testing.T) {
	s := NewServeMux()

	s.Handle("/a", rightHandler)
	req := httptest.NewRequest(http.MethodPost, "/a", nil)

	h, path := s.Handler(req)

	if h != rightHandler {
		t.Error("Wrong handler returned")
	}

	if path != "/a" {
		t.Error("Wrong path, expected /a, got", path)
	}
}

func TestServeMux_MiddlewareSingle(t *testing.T) {
	s := NewServeMux()

	s.Middleware("/", mid1)
	s.Handle("/", rightHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	_, mids, _ := s.HandlerAndMiddleware(req)

	if len(mids) != 1 {
		t.Fatal("Wrong number of middlewares returned. Expected 1, got", len(mids))
	}

	if mids[0] != mid1 {
		t.Error("wat")
	}
}

func TestServeMux_MiddlewareDouble(t *testing.T) {
	s := NewServeMux()

	s.Route("/").
		Middleware(mid1).
		Get(rightHandler).
		Middleware(mid2)

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	_, mids, _ := s.HandlerAndMiddleware(req)

	if len(mids) != 2 {
		t.Fatal("Wrong number of middlewares returned. Expected 2, got", len(mids))
	}

	if mids[0] != mid1 {
		t.Error("Wrong middleware 1")
	}
	if mids[1] != mid2 {
		t.Error("Wrong middleware 2")
	}
}

func TestServeMux_MiddlewareFunc(t *testing.T) {
	s := NewServeMux()

	var called bool

	midFunc := func(res http.ResponseWriter, req *http.Request, next NextMiddlewareFunc) {
		called = true
	}

	s.MiddlewareFunc("/", midFunc)

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	_, mids, _ := s.HandlerAndMiddleware(req)

	if len(mids) != 1 {
		t.Fatal("Wrong number of middlewares returned. Expected 2, got", len(mids))
	}

	s.ServeHTTP(nil, req)

	if !called {
		t.Error("Middleware not called")
	}
}
