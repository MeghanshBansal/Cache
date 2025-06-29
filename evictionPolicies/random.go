package EvictionPolicy

import (
	"math/rand"
)

type RandomPolicy struct {
	keys []string
}

func NewRandomPolicy() Policy {
	return RandomPolicy{
		keys: make([]string, 0),
	}
}

func (r RandomPolicy) RecordAccess(EvictableItem) {
	// no need to track access
}

func (r RandomPolicy) AddItem(item EvictableItem) {
	r.keys = append(r.keys, item.Key())
}

func (r RandomPolicy) RemoveItem(key string) {
	for i, k := range r.keys {
		if k == key {
			r.keys = append(r.keys[:i], r.keys[i+1:]...)
			break
		}
	}
}

func (r RandomPolicy) Evict() string {
	if len(r.keys) == 0 {
		return ""
	}

	return r.keys[rand.Intn(len(r.keys))]
}

func (r RandomPolicy) Name() string {
	return "RANDOM"
}
