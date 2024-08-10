/*
Copyright 2024 The Kubernetes CRSM Authors.

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

	"k8s.io/client-go/dynamic"

	"github.com/rexagod/crsm/internal"
	v "github.com/rexagod/crsm/internal/version"
	clientset "github.com/rexagod/crsm/pkg/generated/clientset/versioned"
	"github.com/rexagod/crsm/pkg/signals"
)

func main() {

	// Set up flags.
	klog.InitFlags(flag.CommandLine)
	options := internal.NewOptions()
	options.Read()

	// Quit if only version flag is set.
	if options.Version && flag.NFlag() == 1 {
		fmt.Println(v.Version())
		os.Exit(0)
	}

	// Set up contextual logging.
	// Set up signals, so we can handle the shutdown signal gracefully.
	ctx := klog.NewContext(signals.SetupSignalHandler(), klog.NewKlogr())
	logger := klog.FromContext(ctx)

	// Build client-sets.
	cfg, err := clientcmd.BuildConfigFromFlags(options.MasterURL, options.Kubeconfig)
	if err != nil {
		logger.Error(err, "Error building kubeconfig", "kubeconfig", options.Kubeconfig)
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
	dynamicClientset, err := dynamic.NewForConfig(cfg)
	if err != nil {
		logger.Error(err, "Error building dynamic clientset")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	// Start the controller.
	if err = internal.NewController(options, kubeClientset, crsmClientset, dynamicClientset).Run(ctx, options.Workers); err != nil {
		logger.Error(err, "Error running controller")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
}
