package main

import (
	"strings"
	"sync"
	"time"
)

func reindexLoop(s *Store, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for range t.C {
		s.Reindex()
	}
}

// Reindex rebuilds the search index, indexing items concurrently.
func (s *Store) Reindex() {
	s.mu.RLock()
	items := make([]Item, 0, len(s.items))
	for _, it := range s.items {
		items = append(items, it)
	}
	s.mu.RUnlock()

	index := make(map[string]int, len(items))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, item := range items {
		item := item // copy for the goroutine below
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := strings.ToLower(strings.TrimSpace(item.Name))
			mu.Lock()
			index[key] = item.ID
			mu.Unlock()
		}()
	}
	wg.Wait()

	s.mu.Lock()
	s.search = index
	s.mu.Unlock()
}
