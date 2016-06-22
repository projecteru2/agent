package utils

import "hash/fnv"

type HashBackends struct {
	data   []string
	length uint32
}

func NewHashBackends(data []string) *HashBackends {
	return &HashBackends{data, uint32(len(data))}
}

func (self *HashBackends) Get(v string, offset int) string {
	h := fnv.New32a()
	h.Write([]byte(v))
	return self.data[(h.Sum32()+uint32(offset))%self.length]
}

func (self *HashBackends) Len() int {
	return len(self.data)
}
