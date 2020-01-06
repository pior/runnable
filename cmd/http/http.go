package main

import (
	"net/http"

	"github.com/pior/runnable"
)

func main() {
	server := &http.Server{
		Addr:    ":80",
		Handler: http.RedirectHandler("https://golang.org/", http.StatusPermanentRedirect),
	}

	runnable.RunGroup(runnable.HTTPServer(server))
}
