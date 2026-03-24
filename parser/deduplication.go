// Package parser - Deduplication utilities
package parser

import (
	"crypto/sha256"
	"fmt"
	"sync"
)

// ChunkDeduplicator handles deduplication of chunks based on content hashing
type ChunkDeduplicator struct {
	seenHashes map[string]bool
	mu         sync.RWMutex
}

// NewChunkDeduplicator creates a new chunk deduplicator
func NewChunkDeduplicator() *ChunkDeduplicator {
	return &ChunkDeduplicator{
		seenHashes: make(map[string]bool),
	}
}

// IsDuplicate checks if a chunk is a duplicate based on its hash
func (cd *ChunkDeduplicator) IsDuplicate(chunk *Chunk) bool {
	cd.mu.RLock()
	defer cd.mu.RUnlock()
	
	return cd.seenHashes[chunk.Hash]
}

// MarkAsSeen marks a chunk's hash as seen
func (cd *ChunkDeduplicator) MarkAsSeen(chunk *Chunk) {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	
	cd.seenHashes[chunk.Hash] = true
}

// DeduplicateChunks removes duplicate chunks from a slice
func (cd *ChunkDeduplicator) DeduplicateChunks(chunks []Chunk) []Chunk {
	unique := make([]Chunk, 0)
	
	for _, chunk := range chunks {
		if !cd.IsDuplicate(&chunk) {
			cd.MarkAsSeen(&chunk)
			unique = append(unique, chunk)
		}
	}
	
	return unique
}

// Reset clears the deduplicator's state
func (cd *ChunkDeduplicator) Reset() {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	
	cd.seenHashes = make(map[string]bool)
}

// GetSeenCount returns the number of unique chunks seen
func (cd *ChunkDeduplicator) GetSeenCount() int {
	cd.mu.RLock()
	defer cd.mu.RUnlock()
	
	return len(cd.seenHashes)
}

// DeduplicateChunksGlobal removes duplicates from a slice without maintaining state
func DeduplicateChunksGlobal(chunks []Chunk) []Chunk {
	seen := make(map[string]bool)
	unique := make([]Chunk, 0)
	
	for _, chunk := range chunks {
		if !seen[chunk.Hash] {
			seen[chunk.Hash] = true
			unique = append(unique, chunk)
		}
	}
	
	return unique
}

// ComputeSimilarityHash computes a similarity hash for fuzzy deduplication
// This uses a simplified simhash algorithm
func ComputeSimilarityHash(content string) string {
	// Simplified simhash: hash of sorted words
	words := SplitIntoWords(content)
	words = RemoveStopWords(words)
	
	// Create a fingerprint from word hashes
	fingerprint := make([]int, 256)
	
	for _, word := range words {
		hash := sha256.Sum256([]byte(word))
		for i, b := range hash {
			if b&1 == 1 {
				fingerprint[i]++
			} else {
				fingerprint[i]--
			}
		}
	}
	
	// Convert fingerprint to hash string
	result := make([]byte, 32)
	for i := 0; i < 256; i += 8 {
		var b byte
		for j := 0; j < 8; j++ {
			if fingerprint[i+j] > 0 {
				b |= 1 << uint(j)
			}
		}
		result[i/8] = b
	}
	
	return fmt.Sprintf("%x", result)
}

// HammingDistance computes the Hamming distance between two hashes
func HammingDistance(hash1, hash2 string) int {
	if len(hash1) != len(hash2) {
		return -1
	}
	
	distance := 0
	for i := 0; i < len(hash1); i++ {
		if hash1[i] != hash2[i] {
			distance++
		}
	}
	
	return distance
}

// AreSimilar checks if two chunks are similar based on their similarity hashes
func AreSimilar(chunk1, chunk2 *Chunk, threshold int) bool {
	hash1 := ComputeSimilarityHash(chunk1.Content)
	hash2 := ComputeSimilarityHash(chunk2.Content)
	
	distance := HammingDistance(hash1, hash2)
	return distance >= 0 && distance <= threshold
}

// FuzzyDeduplicator handles fuzzy deduplication based on similarity
type FuzzyDeduplicator struct {
	chunks    []Chunk
	threshold int
	mu        sync.RWMutex
}

// NewFuzzyDeduplicator creates a new fuzzy deduplicator
func NewFuzzyDeduplicator(threshold int) *FuzzyDeduplicator {
	return &FuzzyDeduplicator{
		chunks:    make([]Chunk, 0),
		threshold: threshold,
	}
}

// AddChunk adds a chunk if it's not similar to existing chunks
func (fd *FuzzyDeduplicator) AddChunk(chunk Chunk) bool {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	
	// Check similarity with existing chunks
	for _, existing := range fd.chunks {
		if AreSimilar(&chunk, &existing, fd.threshold) {
			return false // Similar chunk already exists
		}
	}
	
	// Add chunk if not similar to any existing chunk
	fd.chunks = append(fd.chunks, chunk)
	return true
}

// GetUniqueChunks returns all unique chunks
func (fd *FuzzyDeduplicator) GetUniqueChunks() []Chunk {
	fd.mu.RLock()
	defer fd.mu.RUnlock()
	
	result := make([]Chunk, len(fd.chunks))
	copy(result, fd.chunks)
	return result
}

// Reset clears the fuzzy deduplicator's state
func (fd *FuzzyDeduplicator) Reset() {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	
	fd.chunks = make([]Chunk, 0)
}

// DeduplicationStats provides statistics about deduplication
type DeduplicationStats struct {
	TotalChunks     int     `json:"total_chunks"`
	UniqueChunks    int     `json:"unique_chunks"`
	DuplicateChunks int     `json:"duplicate_chunks"`
	DeduplicationRate float64 `json:"deduplication_rate"`
}

// ComputeDeduplicationStats computes statistics for a deduplication operation
func ComputeDeduplicationStats(original, deduplicated []Chunk) *DeduplicationStats {
	total := len(original)
	unique := len(deduplicated)
	duplicates := total - unique
	
	rate := 0.0
	if total > 0 {
		rate = float64(duplicates) / float64(total) * 100
	}
	
	return &DeduplicationStats{
		TotalChunks:       total,
		UniqueChunks:      unique,
		DuplicateChunks:   duplicates,
		DeduplicationRate: rate,
	}
}
