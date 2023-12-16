package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var (
		ctx, cancel = context.WithCancel(context.Background())
	)
	defer cancel()

	catchSignal(func() {
		cancel()
	})

	<-ctx.Done()
}

func catchSignal(onExit func()) {
	if onExit == nil {
		return
	}

	sign := make(chan os.Signal, 1)
	signal.Notify(sign, syscall.SIGKILL, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for i := 0; ; i++ {
			<-sign
			if i > 0 {
				// force shutdown
				os.Exit(1)
			}
			onExit()
		}
	}()
}
