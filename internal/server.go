/*
Copyright 2024 The Kubernetes crdmetrics Authors.

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

package internal

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/expfmt"
)

// server defines behaviours for a Prometheus-based exposition server.
type server interface {

	// Build sets up the server with the given gatherer.
	build(context.Context, kubernetes.Interface, prometheus.Gatherer) *http.Server
}

// selfServer implements the server interface, and exposes telemetry metrics.
type selfServer struct {
	promHTTPLogger

	// addr is the http.Server address to listen on.
	addr string
}

// mainServer implements the server interface, and exposes resource metrics.
type mainServer struct {
	promHTTPLogger

	// addr is the http.Server address to listen on.
	addr string

	// m is the map of currently active stores per resource.
	m map[types.UID][]*StoreType

	// requestsDurationVec is a histogram denoting the request durations for the metrics endpoint. The metric itself is
	// registered in the telemetry registry, and will be available along with all other main metrics, to not pollute the
	// resource metrics.
	requestsDurationVec *prometheus.ObserverVec
}

// Ensure that selfServer implements the server interface.
var _ server = &selfServer{}

// Ensure that mainServer implements the server interface.
var _ server = &mainServer{}

// newSelfServer returns a new selfServer.
func newSelfServer(addr string) *selfServer {
	return &selfServer{promHTTPLogger{"self"}, addr}
}

// newMainServer returns a new mainServer.
func newMainServer(addr string, m map[types.UID][]*StoreType, requestsDurationVec prometheus.ObserverVec) *mainServer {
	return &mainServer{promHTTPLogger{"main"}, addr, m, &requestsDurationVec}
}

// Build sets up the selfServer with the given gatherer.
func (s *selfServer) build(ctx context.Context, c kubernetes.Interface, g prometheus.Gatherer) *http.Server {
	logger := klog.FromContext(ctx)
	mux := http.NewServeMux()

	// Handle the pprof debug paths.
	mux.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	mux.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))

	// Handle the metrics path.
	metricsHandler := promhttp.HandlerFor(g, promhttp.HandlerOpts{
		ErrorLog:      s.promHTTPLogger,
		ErrorHandling: promhttp.ContinueOnError,
		Registry:      g.(*prometheus.Registry),
	})
	mux.Handle("/metrics", metricsHandler)

	// Handle the readyz path.
	readyzProber := newReadyz(s.source)
	mux.Handle(readyzProber.getAsString(), readyzProber.probe(ctx, logger, c))

	return &http.Server{
		ErrorLog:          log.New(os.Stdout, s.source, log.LstdFlags|log.Lshortfile),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		Addr:              s.addr,
	}
}

// Build sets up the mainServer with the given gatherer.
func (s *mainServer) build(ctx context.Context, c kubernetes.Interface, _ prometheus.Gatherer) *http.Server {
	logger := klog.FromContext(ctx)
	mux := http.NewServeMux()

	// Handle the metrics path.
	var readBinarySemaphore sync.RWMutex
	metricsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		readBinarySemaphore.RLock()
		defer readBinarySemaphore.RUnlock()

		// OpenMetrics is experimental at the moment.
		negotiatedContentType := expfmt.Negotiate(r.Header)
		if negotiatedContentType.FormatType() != expfmt.TypeTextPlain {
			w.Header().Set("Content-Type", string(expfmt.NewFormat(expfmt.TypeTextPlain)))
		}

		// Write out the metrics from all the stores.
		for _, stores := range s.m {
			err := newMetricsWriter(stores...).writeAllTo(w)
			if err != nil {
				logger.Error(err, "error writing metrics", "source", s.source)
			}
		}
	})
	mux.Handle("/metrics", promhttp.InstrumentHandlerDuration(*s.requestsDurationVec, metricsHandler))

	// Handle the healthz path.
	healthzProber := newHealthz(s.source)
	mux.Handle(healthzProber.getAsString(), healthzProber.probe(ctx, logger, c))

	// Handle the livez path.
	livezProber := newLivez(s.source)
	mux.Handle(livezProber.getAsString(), livezProber.probe(ctx, logger, c))

	return &http.Server{
		ErrorLog:          log.New(os.Stdout, s.source, log.LstdFlags|log.Lshortfile),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		Addr:              s.addr,
	}
}

// promHTTPLogger implements promhttp.Logger.
type promHTTPLogger struct {

	// source is the originating server for the log.
	source string
}

// Println logs on all errors received by promhttp.Logger.
func (l promHTTPLogger) Println(v ...interface{}) {
	klog.ErrorS(fmt.Errorf("%s", v), "err", "source", l.source)
}
