package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/mux"
)

// Item is one inventory record.
type Item struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

// Store holds items in memory along with a search index keyed by lowercase name.
type Store struct {
	mu     sync.RWMutex
	items  map[int]Item
	nextID int
	search map[string]int
}

func NewStore() *Store {
	return &Store{items: map[int]Item{}, nextID: 1, search: map[string]int{}}
}

func (s *Store) listItems(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Item, 0, len(s.items))
	for _, it := range s.items {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	writeJSON(w, http.StatusOK, out)
}

func (s *Store) createItem(w http.ResponseWriter, r *http.Request) {
	var in Item
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil || strings.TrimSpace(in.Name) == "" {
		http.Error(w, "invalid item", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	in.ID = s.nextID
	s.nextID++
	s.items[in.ID] = in
	s.mu.Unlock()
	writeJSON(w, http.StatusCreated, in)
}

func (s *Store) getItem(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	s.mu.RLock()
	it, ok := s.items[id]
	s.mu.RUnlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, it)
}

func (s *Store) deleteItem(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	s.mu.Lock()
	_, ok := s.items[id]
	delete(s.items, id)
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Store) reindex(w http.ResponseWriter, r *http.Request) {
	s.Reindex()
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
