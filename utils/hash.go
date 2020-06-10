package utils

import "hash/fnv"

// HashBackends is a simple hash backend
type HashBackends struct {
	data   []string
	length uint32
}

// NewHashBackends new a hash backends
func NewHashBackends(data []string) *HashBackends {
	return &HashBackends{data, uint32(len(data))}
}

// Get get a backend
func (s *HashBackends) Get(v string, offset int) string {
	if s.length == 0 {
		return ""
	}
	h := fnv.New32a()
	if _, err := h.Write([]byte(v)); err != nil {
		return ""
	}
	return s.data[(h.Sum32()+uint32(offset))%s.length]
}

// Len get len of backends
func (s *HashBackends) Len() uint32 {
	return s.length
}
