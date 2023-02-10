package app

import (
	"context"
	"github.com/lmxia/gaia/cmd/gaia-controllers/app/option"
	"github.com/lmxia/gaia/pkg/controllermanager"
	"github.com/lmxia/gaia/pkg/features"
	"github.com/lmxia/gaia/pkg/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

var (
	// the command name
	cmdName = "gaia-controllers"
)

// NewGaiaControllerCmd creates a *cobra.Command object with default parameters
func NewGaiaControllerCmd(ctx context.Context) *cobra.Command {
	opts := option.NewOptions()
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
			cc, agent, err := controllermanager.NewControllerManager(agentCtx, opts.Kubeconfig, opts.ClusterHostName, opts.NetworkBindUrl, opts.ManagedCluster, opts)
			if err != nil {
				klog.Exit(err)
			}
			agent.Run(cc)
		},
	}

	version.AddVersionFlag(cmd.Flags())
	opts.AddFlags(cmd.Flags())
	features.DefaultMutableFeatureGate.AddFlag(cmd.Flags())
	return cmd

}
