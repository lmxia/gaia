package main

import (
	goflag "flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/lmxia/gaia/cmd/gaia-scheduler/app"
	"github.com/lmxia/gaia/pkg/utils"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	klog.InitFlags(nil)
	defer klog.Flush()

	ctx := utils.GracefulStopWithContext()
	scheduleCmd := app.NewGaiaScheduleControllerCmd(ctx)

	pflag.CommandLine.SetNormalizeFunc(utils.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	if err := scheduleCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}
}
