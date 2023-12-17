package app

import (
	"errors"
	"io"
	"log"
	"sync"
)

type App struct {
	bufPool sync.Pool
	once    sync.Once
}

func (a *App) Handle(w io.Writer, r io.Reader) {
	a.once.Do(func() {
		a.bufPool = sync.Pool{New: func() any {
			return make([]byte, 10_000)
		}}
	})

	line := a.bufPool.Get().([]byte)
	defer func() {
		a.bufPool.Put(line[0:cap(line)])
	}()

	n, err := io.ReadFull(r, line)
	if err != nil {
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			log.Println("handle err read.", err)
			return
		}
	}

	line = line[:n]

	var (
		tmp     byte
		lenLine = len(line)
	)

	// don't reverse '\n'
	if line[len(line)-1] == '\n' {
		lenLine = len(line) - 1
	}

	for i := 0; i < lenLine/2; i++ {
		tmp = line[i]
		line[i] = line[lenLine-i-1]
		line[lenLine-i-1] = tmp
	}

	_, err = w.Write(line)
	if err != nil {
		log.Println("handle err write.", err)
		return
	}
}
