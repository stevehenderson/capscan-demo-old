package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoutes(t *testing.T) {
	s := NewStore()
	h := newRouter(s, "secret")

	tests := []struct {
		name, method, path, body, auth string
		want                           int
	}{
		{"create", "POST", "/items", `{"name":"widget","quantity":3}`, "", http.StatusCreated},
		{"list", "GET", "/items", "", "", http.StatusOK},
		{"get", "GET", "/items/1", "", "", http.StatusOK},
		{"get missing", "GET", "/items/99", "", "", http.StatusNotFound},
		{"non-numeric id", "GET", "/items/abc", "", "", http.StatusNotFound},
		{"encoded slash", "GET", "/items/..%2F..%2Fetc%2Fpasswd", "", "", http.StatusMovedPermanently},
		{"wrong method", "PUT", "/items/1", "", "", http.StatusMethodNotAllowed},
		{"admin without token", "POST", "/admin/reindex", "", "", http.StatusUnauthorized},
		{"admin with wrong token", "POST", "/admin/reindex", "", "Bearer nope", http.StatusUnauthorized},
		{"admin with token", "POST", "/admin/reindex", "", "Bearer secret", http.StatusNoContent},
		{"delete", "DELETE", "/items/1", "", "", http.StatusNoContent},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("%s %s = %d, want %d", tc.method, tc.path, rec.Code, tc.want)
			}
		})
	}
}

func TestReindex(t *testing.T) {
	s := NewStore()
	for _, n := range []string{"Widget", "Gadget", "Sprocket"} {
		s.items[s.nextID] = Item{ID: s.nextID, Name: n}
		s.nextID++
	}
	s.Reindex()
	if got := s.search["gadget"]; got != 2 {
		t.Fatalf("search[gadget] = %d, want 2", got)
	}
}
