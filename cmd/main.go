package main

import (
	"context"
	"fmt"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/reddit/achilles-sdk/pkg/bootstrap"
	"github.com/reddit/achilles-sdk/pkg/fsm/metrics"
	"github.com/reddit/achilles-sdk/pkg/io"
	"github.com/reddit/achilles-sdk/pkg/logging"
	"github.com/reddit/achilles-sdk/pkg/meta"
	"github.com/reddit/achilles-sdk/pkg/ratelimiter"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	kubeconfig "github.com/klaudworks/kubeconfig-operator/internal/controllers/kubeconfig"
	"github.com/klaudworks/kubeconfig-operator/internal/controlplane"
	intscheme "github.com/klaudworks/kubeconfig-operator/internal/scheme"
)

// opts store any optional settings that instruct how the manager and
// controllers should run. Typically these are fed values from CLI flags or
// environment variables.
type opts struct {
	bootstrap   bootstrap.Options
	disableSync bool
}

const (
	ApplicationName = "kubeconfig-operator"
	ComponentName   = "kubeconfig-operator"
)

// Version is dynamically set at compile time
var Version = "0.0.1"

func main() {
	ctx := context.Background()
	if err := rootCommand(ctx).Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %s\n", err)
		os.Exit(1)
	}
}

func rootCommand(ctx context.Context) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:     "kubeconfig-operator",
		Short:   "Start the GlobalConfig controller manager",
		Version: "",
		RunE: func(cmd *cobra.Command, args []string) error {
			return bootstrap.Start(ctx,
				intscheme.AddToSchemes,
				&o.bootstrap,
				initStartFunc(o))
		},
	}
	o.addToFlags(cmd.Flags())

	return cmd
}

func (o *opts) addToFlags(flags *pflag.FlagSet) {
	o.bootstrap.AddToFlags(flags)

	flags.BoolVar(&o.disableSync, "disable-sync", false, "run controllers in a dry-run mode (default: false)")
}

// initStartFunc accepts options that are typically set from CLI flags or
// environment variables. It returns an instance of [bootstrap.StartFunc],
// which can then be fed into [bootstrap.Start].
func initStartFunc(o *opts) bootstrap.StartFunc {
	return func(ctx context.Context, mgr manager.Manager) error {
		rl := ratelimiter.NewDefaultProviderRateLimiter(ratelimiter.DefaultProviderRPS)
		meta.InitRedditLabels(ApplicationName, Version, ComponentName)

		client := &io.ClientApplicator{
			Client:     mgr.GetClient(),
			Applicator: io.NewAPIPatchingApplicator(mgr.GetClient()),
		}

		// metrics sink
		promReg := prometheus.NewRegistry()
		promMetrics := metrics.MustMakeMetrics(mgr.GetScheme(), promReg)

		// map flag values into controlplane's context
		cpCtx := controlplane.Context{
			DisableSync: o.disableSync,
			Metrics:     promMetrics,
		}
		log, err := logging.FromContext(ctx)
		if err != nil {
			return fmt.Errorf("getting logger from context: %w", err)
		}

		log.Info("starting controllers...")
		if err := kubeconfig.SetupController(ctx, cpCtx, mgr, rl, client); err != nil {
			return fmt.Errorf("setting up Kubeconfig controller: %w", err)
		}
		return nil
	}
}
