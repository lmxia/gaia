package app

import (
	"context"
	"gaia.io/gaia/pkg/controllermanager"
	"gaia.io/gaia/pkg/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

var (
	// the command name
	cmdName = "gaia-controllers"
)

// NewClusternetAgentCmd creates a *cobra.Command object with default parameters
func NewGaiaControllerCmd(ctx context.Context) *cobra.Command {
	opts := NewOptions()
	cmd := &cobra.Command{
		Use:  cmdName,
		Long: `Responsible for cluster registration, cluster metric collecting and report, etc`,
		Run: func(cmd *cobra.Command, args []string) {
			opts.Complete()

			if err := opts.Validate(); err != nil {
				klog.Exit(err)
			}

			cmd.Flags().VisitAll(func(flag *pflag.Flag) {
				klog.V(1).Infof("FLAG: --%s=%q", flag.Name, flag.Value)
			})

			// TODO: add logic
			agentCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			agent, err := controllermanager.NewControllerManager(agentCtx, opts.kubeconfig)
			if err != nil {
				klog.Exit(err)
			}
			agent.Run()
		},
	}

	version.AddVersionFlag(cmd.Flags())
	return cmd

}