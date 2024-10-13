package util

import "sync"

var (
	poolsLock sync.RWMutex
	pools     = map[int]*sync.Pool{}
)

func PoolGet(size int) []byte {
	poolsLock.RLock()
	pool, found := pools[size]
	poolsLock.RUnlock()
	if !found || pool == nil {
		return make([]byte, size)
	}
	buf, ok := pool.Get().([]byte)
	if buf == nil || !ok {
		buf = make([]byte, size)
	}
	return buf[:size]
}

func PoolPut(buf []byte) {
	size := cap(buf)
	poolsLock.Lock()
	pool, found := pools[size]
	if !found {
		pool = &sync.Pool{New: func() any { return make([]byte, size) }}
		pools[size] = pool
	}
	poolsLock.Unlock()
	pool.Put(buf)
}
