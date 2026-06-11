package main

import "strings"

// cacheKeyForStage builds the canonical cache entry value: "<hash>|<command>".
func cacheKeyForStage(stage Stage, hash string) string {
	if hash == "" {
		return ""
	}
	return hash + "|" + strings.Join(stage.Cmd, " ")
}

// cacheHit reports whether the stage is cached for the given content hash.
// Legacy entries stored as hash-only (without "|command") still match.
func cacheHit(cache map[string]string, stage Stage, hash string) bool {
	if hash == "" {
		return false
	}
	entry, ok := cache[stage.Name]
	if !ok {
		return false
	}
	key := cacheKeyForStage(stage, hash)
	if entry == key {
		return true
	}
	return entry == hash
}
