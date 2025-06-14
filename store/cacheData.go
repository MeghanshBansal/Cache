package store

import "time"

type CacheData struct {
	Value   any
	TTL     time.Duration
	Created time.Time
}

func (c *CacheData) HasExpired() bool {
	return time.Now().Sub(c.Created) > c.TTL
}
