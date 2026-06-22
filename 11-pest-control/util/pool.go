package util

import (
	"sync"
)

var (
	poolsBytes     = map[int]*sync.Pool{}
	poolsBytesLock sync.RWMutex
)

func GetBytes(size int) []byte {
	poolsBytesLock.RLock()
	pool, found := poolsBytes[size]
	poolsBytesLock.RUnlock()
	if !found || pool == nil {
		return make([]byte, size)
	}
	buf, ok := pool.Get().([]byte)
	if buf == nil || !ok {
		buf = make([]byte, size)
	}
	return buf[:size]
}

func PutBytes(buf []byte) {
	size := cap(buf)
	poolsBytesLock.Lock()
	pool, found := poolsBytes[size]
	if !found {
		pool = &sync.Pool{}
		poolsBytes[size] = pool
	}
	poolsBytesLock.Unlock()
	pool.Put(buf)
}
