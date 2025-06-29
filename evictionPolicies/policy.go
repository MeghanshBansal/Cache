package EvictionPolicy

import "time"

type EvictableItem interface {
	Key() string
	CreatedAt() time.Time
	LastUsed() time.Time
	AccessCnt() int
}

type Policy interface {
	// updates the policies internal state on item access
	RecordAccess(EvictableItem)
	// add the item to the policy state tracker
	AddItem(EvictableItem)
	// remove the item from policy state tracker
	RemoveItem(string)
	// returns the key deemed by policy to be removed
	Evict() string
	// return the name of the policy for itentification
	Name() string
}
