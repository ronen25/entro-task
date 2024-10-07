package main

import (
	"log"
	"sync"
)

type ResultCache struct {
	Results map[string][]CommitErrorResult
	mu      sync.Mutex
}

func NewResultCache() ResultCache {
	return ResultCache{
		Results: make(map[string][]CommitErrorResult),
	}
}

func (cache *ResultCache) AddResult(key string, results []CommitErrorResult) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.Results[key] = append(cache.Results[key], results...)
	log.Printf("Added %d entries", len(results))
}

func (cache *ResultCache) GetResults(key string) ([]CommitErrorResult, bool) {
	cache.mu.Lock()
	value, found := cache.Results[key]
	cache.mu.Unlock()

	return value, found
}
