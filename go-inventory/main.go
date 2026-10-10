// Command go-inventory serves a small in-memory inventory API.
package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	store := NewStore()
	go reindexLoop(store, time.Minute)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           newRouter(store, os.Getenv("ADMIN_TOKEN")),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}
