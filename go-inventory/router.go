package main

import (
	"crypto/subtle"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func newRouter(s *Store, adminToken string) http.Handler {
	r := mux.NewRouter()
	r.Use(logRequests)

	r.HandleFunc("/items", s.listItems).Methods(http.MethodGet)
	r.HandleFunc("/items", s.createItem).Methods(http.MethodPost)
	r.HandleFunc("/items/{id:[0-9]+}", s.getItem).Methods(http.MethodGet)
	r.HandleFunc("/items/{id:[0-9]+}", s.deleteItem).Methods(http.MethodDelete)

	admin := r.PathPrefix("/admin").Subrouter()
	admin.Use(requireToken(adminToken))
	admin.HandleFunc("/reindex", s.reindex).Methods(http.MethodPost)

	return r
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// requireToken rejects requests without the admin bearer token.
// An empty token disables the admin routes entirely.
func requireToken(token string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := r.Header.Get("Authorization")
			want := "Bearer " + token
			if token == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
