package main

import (
	goflag "flag"
	"fmt"
	"github.com/lmxia/gaia/cmd/gaia-controllers/app"
	"github.com/lmxia/gaia/pkg/utils"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
	"math/rand"
	"net/http"
	"os"
	"time"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	klog.InitFlags(nil)
	defer klog.Flush()

	ctx := utils.GracefulStopWithContext()
	command := app.NewGaiaControllerCmd(ctx)
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2113", nil)

	pflag.CommandLine.SetNormalizeFunc(utils.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
