/*
Copyright 2023 The Kubernetes crsm Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/rexagod/crsm/internal"
	v "github.com/rexagod/crsm/internal/version"
	clientset "github.com/rexagod/crsm/pkg/generated/clientset/versioned"
	"github.com/rexagod/crsm/pkg/signals"
)

func main() {

	// Set up flags.
	klog.InitFlags(nil)
	klog.SetOutput(os.Stdout)
	kubeconfig := *flag.String("kubeconfig", os.Getenv("KUBECONFIG"), "Path to a kubeconfig. Only required if out-of-cluster.")
	masterURL := *flag.String("master", os.Getenv("KUBERNETES_MASTER"), "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	workers := *flag.Int("workers", 2, "Number of workers processing the queue. Defaults to 2.")
	version := *flag.Bool("version", false, "Print version information and quit")
	flag.Parse()

	// Quit if only version flag is set.
	if version && flag.NFlag() == 1 {
		fmt.Println(v.Version())
		os.Exit(0)
	}

	// Set up contextual logging.
	// Set up signals, so we can handle the shutdown signal gracefully.
	ctx := klog.NewContext(signals.SetupSignalHandler(), klog.NewKlogr())
	logger := klog.FromContext(ctx)

	// Build client-sets.
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		logger.Error(err, "Error building kubeconfig", "kubeconfig", kubeconfig)
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
	kubeClientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.Error(err, "Error building kubernetes clientset")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
	crsmClientset, err := clientset.NewForConfig(cfg)
	if err != nil {
		logger.Error(err, "Error building crsm clientset")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	// Start the controller.
	if err = internal.NewController(ctx, kubeClientset, crsmClientset).Run(ctx, workers); err != nil {
		logger.Error(err, "Error running controller")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
}
