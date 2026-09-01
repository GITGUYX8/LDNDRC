// Package gateway proxies WebSocket and HTTP traffic from the control panel
// into a workspace pod. A single session pod hosts three services (editor on
// 7682, desktop on 8080, gazebo on 9002), so the proxy target is chosen per
// route.
//
// Go's httputil.ReverseProxy handles both plain HTTP and WebSocket upgrade
// requests, which is what the browser-based code-server, gzweb, and Selkies
// clients need.
package gateway

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// Target describes one proxied service inside a session pod.
type Target struct {
	// Name is the route segment, e.g. "editor", "desktop", "sim".
	Name string
	// URL is the pod endpoint, e.g. http://10.42.3.5:7682.
	URL *url.URL
}

// Handler returns an HTTP handler that proxies matching named routes to their
// pod targets. Routes are resolved per request so a session can be mapped
// dynamically (the sessions package supplies targets).
func Handler(resolve func(r *http.Request) (*url.URL, bool)) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: "placeholder"})
	proxy.Director = func(req *http.Request) {
		target, ok := resolve(req)
		if !ok {
			return
		}
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
	}
	return proxy
}
