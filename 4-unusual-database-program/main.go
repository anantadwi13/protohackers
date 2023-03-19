package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
)

func main() {
	var (
		wgServer  = sync.WaitGroup{}
		signChan  = make(chan os.Signal, 1)
		db        = &Database{val: map[string]string{}}
		maxWorker = runtime.NumCPU()
	)

	signal.Notify(signChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)

	addr, err := net.ResolveUDPAddr("udp", ":8000")
	if err != nil {
		panic(err)
	}
	listener, err := net.ListenUDP("udp", addr)
	if err != nil {
		panic(err)
	}

	wgServer.Add(maxWorker)
	for i := 0; i < maxWorker; i++ {
		go func(i int) {
			defer wgServer.Done()

			buf := make([]byte, 1024)
			for {
				n, udpAddr, err := listener.ReadFromUDP(buf)
				if err != nil {
					return
				}
				handlePacket(db, listener, udpAddr, buf[:n])
			}
		}(i)
	}

	<-signChan
	_ = listener.Close()
	wgServer.Wait()
}

type Database struct {
	val  map[string]string
	lock sync.RWMutex
}

func handlePacket(db *Database, udpConn *net.UDPConn, addr *net.UDPAddr, packet []byte) {
	packetSplit := strings.SplitN(string(packet), "=", 2)

	switch len(packetSplit) {
	case 2:
		// write
		key, val := packetSplit[0], packetSplit[1]

		if key == "version" {
			return
		}
		db.lock.Lock()
		db.val[key] = val
		db.lock.Unlock()
	case 1:
		// read
		key := packetSplit[0]

		val := ""
		if key == "version" {
			val = "Ken's Key-Value Store 1.0"
		} else {
			db.lock.RLock()
			val = db.val[key]
			db.lock.RUnlock()
		}

		_, err := udpConn.WriteToUDP([]byte(fmt.Sprintf("%s=%s", key, val)), addr)
		if err != nil {
			log.Println("write err: ", err)
			return
		}
	default:
		return
	}
}
