package utils

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/klog/v2"
)

var gracefulStopCh = make(chan os.Signal, 2)

// GracefulStopWithContext registered for SIGTERM and SIGINT with a cancel context returned.
func GracefulStopWithContext() context.Context {
	signal.Notify(gracefulStopCh, syscall.SIGTERM, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		// waiting for os signal to stop the program
		oscall := <-gracefulStopCh
		klog.Warningf("shutting down, caused by %s", oscall)
		cancel()
		<-gracefulStopCh
		os.Exit(1)
	}()

	return ctx
}
