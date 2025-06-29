package EvictionPolicy

import "container/list"

type LRUPolicy struct {
	list  *list.List
	items map[string]*list.Element
}

func NewLRUPolicy() Policy {
	return LRUPolicy{
		list:  list.New(),
		items: make(map[string]*list.Element),
	}
}

func (l LRUPolicy) RecordAccess(item EvictableItem) {
	if item, ok := l.items[item.Key()]; ok {
		l.list.MoveToFront(item)
	}
}

func (l LRUPolicy) AddItem(item EvictableItem) {
	element := l.list.PushFront(item)
	l.items[item.Key()] = element
}

func (l LRUPolicy) RemoveItem(key string) {
	if element, ok := l.items[key]; ok {
		l.list.Remove(element)
		delete(l.items, key)
	}
}

func (l LRUPolicy) Evict() string {
	if element := l.list.Back(); element != nil {
		if item, ok := element.Value.(EvictableItem); ok {
			return item.Key()
		}
	}

	return ""
}

func (l LRUPolicy) Name() string {
	return "LRU"
}
