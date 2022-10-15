package cache

import (
	"time"

	gocache "github.com/patrickmn/go-cache"
)

const (
	NoExpiration = gocache.NoExpiration
	// For use with functions that take an expiration time. Equivalent to
	// passing in the same expiration duration as was given to New() or
	// NewFrom() when the cache was created (e.g. 5 minutes.)
	DefaultExpiration = gocache.DefaultExpiration
)

// 内存k-v数据库
type Cache struct {
	*gocache.Cache
	rcdCh chan interface{}
}

// 内存数据库，大并发系统，有效期设置过大会导致磁盘、内存开销大
func NewCache(def, interval time.Duration) *Cache {
	var c Cache
	c.Cache = gocache.New(def, interval)
	c.rcdCh = make(chan interface{}, 128) // 缓冲128
	return &c
}

func (self *Cache) Close() {
	close(self.rcdCh)
}

func (self *Cache) Input() chan<- interface{} {
	return self.rcdCh
}

func (self *Cache) Output() <-chan interface{} {
	return self.rcdCh
}
