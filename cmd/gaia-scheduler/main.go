package main

import (
	goflag "flag"
	"fmt"
	"os"

	"github.com/lmxia/gaia/cmd/gaia-scheduler/app"
	"github.com/lmxia/gaia/pkg/utils"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

func main() {
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
