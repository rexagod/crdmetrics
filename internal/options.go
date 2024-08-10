package internal

import (
	"flag"
	"os"
)

// Options represents the command-line Options.
type Options struct {
	Kubeconfig string
	MainHost   string
	MainPort   int
	MasterURL  string
	SelfHost   string
	SelfPort   int
	TryNoCache bool
	Version    bool
	Workers    int
	V          int
}

// NewOptions returns a new Options.
func NewOptions() *Options {
	return &Options{}
}

// parseOptions parses the command-line options.
func (o *Options) Read() {
	o.Kubeconfig = *flag.String("kubeconfig", os.Getenv("KUBECONFIG"), "Path to a kubeconfig. Only required if out-of-cluster.")
	o.MainHost = *flag.String("main-host", "::", "Host to expose main metrics on.")
	o.MainPort = *flag.Int("main-port", 9999, "Port to expose main metrics on.")
	o.MasterURL = *flag.String("master", os.Getenv("KUBERNETES_MASTER"), "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	o.SelfHost = *flag.String("self-host", "::", "Host to expose self (telemetry) metrics on.")
	o.SelfPort = *flag.Int("self-port", 9998, "Port to expose self (telemetry) metrics on.")
	o.TryNoCache = *flag.Bool("try-no-cache", false, "In case of stale data, force the API server to [GET/LIST] the most recent resources and ignore the cache. Defaults to false.")
	o.Version = *flag.Bool("version", false, "Print version information and quit")
	o.Workers = *flag.Int("workers", 2, "Number of workers processing the queue. Defaults to 2.")
	flag.Parse()
}
