package app

import (
	"context"

	"github.com/lmxia/gaia/pkg/scheduler"
	"github.com/lmxia/gaia/pkg/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

var (
	// the command name
	cmdName = "gaia-scheduler"
)

// NewGaiaScheduleCmd creates a *cobra.Command object with default parameters
func NewGaiaScheduleControllerCmd(ctx context.Context) *cobra.Command {
	opts := NewOptions()
	cmd := &cobra.Command{
		Use:  cmdName,
		Long: `Responsible for cluster workloads schedule, etc`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Flags().VisitAll(func(flag *pflag.Flag) {
				klog.V(1).Infof("FLAG: --%s=%q", flag.Name, flag.Value)
			})

			// TODO: add logic
			agentCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			agentSchedule, err := scheduler.NewSchedule(agentCtx, opts.kubeconfig)
			if err != nil {
				klog.Exit(err)
			}
			agentSchedule.Run(agentCtx)
		},
	}

	version.AddVersionFlag(cmd.Flags())
	opts.AddFlags(cmd.Flags())
	return cmd
}
