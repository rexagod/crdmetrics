package internal

import (
	"flag"
	"os"
)

// Options represents the command-line Options.
type Options struct {
	AutoGOMAXPROCS  bool
	RatioGOMEMLIMIT float64
	Kubeconfig      string
	MasterURL       string
	SelfHost        string
	SelfPort        int
	MainHost        string
	MainPort        int
	TryNoCache      bool
	Workers         int
	Version         bool
}

// NewOptions returns a new Options.
func NewOptions() *Options {
	return &Options{}
}

// parseOptions parses the command-line options.
func (o *Options) Read() {
	o.AutoGOMAXPROCS = *flag.Bool("auto-gomaxprocs", true, "Automatically set GOMAXPROCS to match CPU quota.")
	o.RatioGOMEMLIMIT = *flag.Float64("ratio-gomemlimit", 0.9, "GOMEMLIMIT to memory quota ratio.")
	o.Kubeconfig = *flag.String("kubeconfig", os.Getenv("KUBECONFIG"), "Path to a kubeconfig. Only required if out-of-cluster.")
	o.MasterURL = *flag.String("master", os.Getenv("KUBERNETES_MASTER"), "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	o.SelfHost = *flag.String("self-host", "::", "Host to expose self (telemetry) metrics on.")
	o.SelfPort = *flag.Int("self-port", 9998, "Port to expose self (telemetry) metrics on.")
	o.MainHost = *flag.String("main-host", "::", "Host to expose main metrics on.")
	o.MainPort = *flag.Int("main-port", 9999, "Port to expose main metrics on.")
	o.TryNoCache = *flag.Bool("try-no-cache", false, "Force the API server to [GET/LIST] the most recent versions.")
	o.Workers = *flag.Int("workers", 2, "Number of workers processing the queue.")
	o.Version = *flag.Bool("version", false, "Print version information and quit")
	flag.Parse()
}
