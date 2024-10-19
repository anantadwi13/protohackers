package util

import (
	"bufio"
	"sync"
)

var (
	poolsBytes         = map[int]*sync.Pool{}
	poolsBytesLock     sync.RWMutex
	poolsBufReader     = map[int]*sync.Pool{}
	poolsBufReaderLock = sync.RWMutex{}
)

func GetBufferReader(size int) *bufio.Reader {
	poolsBufReaderLock.RLock()
	pool, found := poolsBufReader[size]
	poolsBufReaderLock.RUnlock()
	if !found || pool == nil {
		return bufio.NewReaderSize(nil, size)
	}

	reader, ok := pool.Get().(*bufio.Reader)
	if !ok || reader == nil {
		reader = bufio.NewReaderSize(nil, size)
	}
	return reader
}

func PutBufferReader(buf *bufio.Reader) {
	if buf == nil {
		return
	}
	buf.Reset(nil)

	poolsBufReaderLock.Lock()
	pool, found := poolsBufReader[buf.Size()]
	if !found {
		pool = &sync.Pool{}
		poolsBufReader[buf.Size()] = pool
	}
	poolsBufReaderLock.Unlock()
	pool.Put(buf)
}

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
