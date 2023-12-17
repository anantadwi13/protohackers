package main

import (
	"context"
	"github.com/anantadwi13/protohackers/7-line-reversal/app"
	"github.com/anantadwi13/protohackers/7-line-reversal/lrcp"
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

	appHandler := &app.App{}
	l := lrcp.NewLRCP(ctx, nil, appHandler)
	_ = l

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
