package store

import "time"

type CacheData struct {
	key       string
	value     any
	ttl       time.Duration
	createdAt time.Time
	lastUsed  time.Time
	accessCnt int
}

func (c *CacheData) Key() string          { return c.key }
func (c *CacheData) CreatedAt() time.Time { return c.createdAt }
func (c *CacheData) LastUsed() time.Time  { return c.lastUsed }
func (c *CacheData) AccessCnt() int       { return c.accessCnt }
