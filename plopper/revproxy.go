package plopper

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

func revproxy(dest string) http.Handler {
	target, err := url.Parse(dest)
	if err != nil {
		panic(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)

	director := proxy.Director
	proxy.Director = func(req *http.Request) {
		director(req)

		// This is a known annoyance https://github.com/golang/go/issues/7682
		req.Host = target.Host

		req.Header.Set("host", target.Host)
		req.Header.Set("user-agent", "application/plopper")
	}
	return proxy
}
