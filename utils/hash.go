package utils

import "hash/fnv"

type HashBackends struct {
	data []string
}

func NewHashBackends(data []string) *HashBackends {
	return &HashBackends{data: data}
}

func (s *HashBackends) Get(v string) string {
	if len(s.data) == 0 {
		return ""
	}
	h := fnv.New32a()
	if _, err := h.Write([]byte(v)); err != nil {
		return ""
	}
	return s.data[int(h.Sum32())%len(s.data)]
}
