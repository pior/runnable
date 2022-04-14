package main

import (
	"log"
	"net/http"
	"time"

	"github.com/pior/runnable"
)

func main() {
	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		select {
		case <-ctx.Done():
			log.Print("Task interrupted")

		case <-time.After(time.Second * 10):
			log.Print("Task completed")
		}
	}

	server := &http.Server{
		Addr:    "127.0.0.1:8000",
		Handler: http.HandlerFunc(handlerFunc),
	}

	runnable.Run(runnable.HTTPServer(server))
}
