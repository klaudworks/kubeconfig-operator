// Package controlplane contains state shared across all reconcilers.
package controlplane

import "github.com/reddit/achilles-sdk/pkg/fsm/metrics"

// Context holds information on how the controller should run. These values may
// be referenced during the execution of transition functions.
type Context struct {
	// DisableSync, if true, disables this controller from enforcing desired state.
	DisableSync bool

	// Metrics is the prometheus metrics sink for this controller binary.
	Metrics *metrics.Metrics
}
