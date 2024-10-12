package main

import (
	"context"
	"github.com/anantadwi13/protohackers/7-line-reversal/lrcp"
	"github.com/anantadwi13/protohackers/7-line-reversal/util"
	"io"
	"log"
	"os"
	"os/signal"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	server, err := lrcp.NewServer(":19090", func(w io.Writer, r io.Reader) {
		buf := util.PoolGet(lrcp.MaxLineLength)
		defer util.PoolPut(buf)

		n, err := r.Read(buf)
		if err != nil {
			log.Printf("handler err:%s", err)
			return
		}
		buf = buf[:n]

		bufChar := make([]byte, 1)
		for idx := range buf {
			bufChar[0] = buf[len(buf)-1-idx]
			n, err = w.Write(bufChar)
			if err != nil {
				log.Printf("handler err:%s", err)
				return
			}
			if n != 1 {
				log.Printf("handler err:%s", "inconsistent write")
				return
			}
		}

		log.Printf("handler success. %s", buf)
	})
	if err != nil {
		log.Fatalln(err)
	}
	go func() {
		log.Println("listening")
		err = server.Listen()
		if err != nil {
			log.Fatalln(err)
		}
	}()

	<-ctx.Done()
	err = server.Shutdown(context.TODO())
	if err != nil {
		log.Fatalln(err)
	}
}
