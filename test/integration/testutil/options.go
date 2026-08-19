package testutil

import (
	"testing"

	"github.com/yf-networks/ai-gateway-controller/internal/option"
	"github.com/yf-networks/ai-gateway-controller/internal/option/externalLB"
)

// DefaultOpts returns an option.Options with sensible defaults for in-process
// integration tests (matching the controller's documented defaults).
func DefaultOpts() *option.Options {
	opts := option.NewOptions()
	opts.ProductName = "AI_product"
	opts.ClusterName = "testk8s"
	opts.EnableRsPool = true
	opts.EnableInferencePool = true
	opts.NamespaceList = []string{"*"}
	opts.SkipNilSvcDelete = true
	opts.ForceRmFinalizer = false
	opts.ExternalLB = externalLB.NewOptions()
	opts.ExternalLB.Token = "Token testtoken"
	opts.ExternalLB.Timeout = 3000
	return opts
}

// SetOpts installs opts as the global option.Opts and restores the original on
// test cleanup. Tests that touch option.Opts (e.g. filter tests) must use this
// (or NewEnv) and must NOT use t.Parallel().
func SetOpts(t *testing.T, opts *option.Options) {
	t.Helper()
	orig := option.Opts
	option.Opts = opts
	t.Cleanup(func() { option.Opts = orig })
}
