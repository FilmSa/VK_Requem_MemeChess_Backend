package search

import "meme_chess/internal/analyzer/position"

type Bound uint8

const (
	BoundExact Bound = iota
	BoundLower
	BoundUpper
)

type TTEntry struct {
	Hash     uint64
	Depth    int
	Score    int
	Bound    Bound
	BestMove position.Move
}

type TranspositionTable interface {
	Get(hash uint64) (TTEntry, bool)
	Put(entry TTEntry)
}

type MemoryTranspositionTable struct {
	entries map[uint64]TTEntry
}

func NewTranspositionTable() *MemoryTranspositionTable {
	return &MemoryTranspositionTable{
		entries: make(map[uint64]TTEntry),
	}
}

func (t *MemoryTranspositionTable) Get(hash uint64) (TTEntry, bool) {
	entry, ok := t.entries[hash]
	return entry, ok
}

func (t *MemoryTranspositionTable) Put(entry TTEntry) {
	current, ok := t.entries[entry.Hash]
	if ok && current.Depth > entry.Depth {
		return
	}

	t.entries[entry.Hash] = entry
}
