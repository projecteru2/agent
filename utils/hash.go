package utils

import "hash/fnv"

type HashBackends struct {
	data   []string
	length uint32
}

func NewHashBackends(data []string) *HashBackends {
	return &HashBackends{data, uint32(len(data))}
}

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

func (s *HashBackends) Len() uint32 {
	return s.length
}
