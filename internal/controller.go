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

package internal

import (
	"context"
	stderrors "errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"strconv"
	"time"

	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rexagod/crsm/internal/version"
	"github.com/rexagod/crsm/pkg/apis/crsm/v1alpha1"
	clientset "github.com/rexagod/crsm/pkg/generated/clientset/versioned"
	crsmscheme "github.com/rexagod/crsm/pkg/generated/clientset/versioned/scheme"
	informers "github.com/rexagod/crsm/pkg/generated/informers/externalversions"
)

// Controller is the controller implementation for CRSMR resources.
type Controller struct {

	// kubeclientset is a standard kubernetes clientset, required for native operations.
	kubeclientset kubernetes.Interface

	// crsmClientset is a clientset for our own API group.
	crsmClientset clientset.Interface

	// dynamicClientset is a clientset for CRD operations.
	dynamicClientset dynamic.Interface

	// crsmInformerFactory is a shared informer factory for CRSM resources.
	crsmInformerFactory informers.SharedInformerFactory

	// workqueue is a rate limited work queue. This is used to queue work to be processed instead of performing it as
	// soon as a change happens. This means we can ensure we only process a fixed amount of resources at a time, and
	// makes it easy to ensure we are never processing the same item simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface

	// recorder is an event recorder for recording event resources.
	recorder record.EventRecorder

	// crsmUIDToStores is the handler's internal stores map. It records all stores associated with a CRSM resource.
	crsmUIDToStores map[types.UID][]*StoreType

	// options is the collection of command-line options.
	options *Options
}

// NewController returns a new sample controller.
func NewController(ctx context.Context, options *Options, kubeClientset kubernetes.Interface, crsmClientset clientset.Interface, dynamicClientset dynamic.Interface) *Controller {
	logger := klog.FromContext(ctx)

	// Add native resources to the default Kubernetes Scheme so Events can be logged for them.
	utilruntime.Must(crsmscheme.AddToScheme(scheme.Scheme))

	// Initialize the controller.
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClientset.CoreV1().Events(os.Getenv("POD_NAMESPACE") /* emit in the default namespace if none is defined */)})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: version.ControllerName})
	ratelimiter := workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 5*time.Minute),
		&workqueue.BucketRateLimiter{Limiter:
		// Burst is the maximum number of tokens
		// that can be consumed in a single call
		// to Allow, Reserve, or Wait, so higher
		// Burst values allow more events to
		// happen at once. A zero Burst allows no
		// events, unless limit == Inf.
		rate.NewLimiter(rate.Limit(50), 300)},
	)

	controller := &Controller{
		kubeclientset:       kubeClientset,
		crsmClientset:       crsmClientset,
		dynamicClientset:    dynamicClientset,
		crsmInformerFactory: informers.NewSharedInformerFactory(crsmClientset, 0),
		workqueue:           workqueue.NewRateLimitingQueue(ratelimiter),
		recorder:            recorder,
		options:             options,
	}

	// Set up event handlers for CRSMR resources.
	_, err := controller.crsmInformerFactory.Crsm().V1alpha1().CustomResourceStateMetricsResources().Informer().
		AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				controller.enqueueCRSMResource(obj, addEvent)
			},
			UpdateFunc: func(old, new interface{}) {
				oldCRSMR := old.(*v1alpha1.CustomResourceStateMetricsResource)
				newCRSMR := new.(*v1alpha1.CustomResourceStateMetricsResource)
				if oldCRSMR.ResourceVersion == newCRSMR.ResourceVersion ||

					// NOTE: Don't add to workqueue if the event stemmed from a status update, else this will create a
					// reconciliation loop; the resource status update triggers the informer which in turn triggers a
					// reconciliation (with an update event) which again updates the resource status and so on. This
					// also applies to other non-spec fields that are updated, such as labels, but those are handled in
					// the event handler.
					// Queue only for `spec` changes.
					reflect.DeepEqual(oldCRSMR.Spec, newCRSMR.Spec) {
					logger.V(10).Info("Skipping event", "[-old +new]", cmp.Diff(oldCRSMR, newCRSMR))
					return
				}
				logger.V(4).Info("Update event", "[-old +new]", cmp.Diff(oldCRSMR.Spec.ConfigurationYAML, newCRSMR.Spec.ConfigurationYAML))
				controller.enqueueCRSMResource(new, updateEvent)
			},
			DeleteFunc: func(obj interface{}) {
				controller.enqueueCRSMResource(obj, deleteEvent)
			},
		})
	if err != nil {
		logger.Error(err, "error setting up event handlers")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	return controller
}

// enqueueCRSMResource takes a CRSMR resource and converts it into a namespace/name key.
func (c *Controller) enqueueCRSMResource(obj interface{}, event eventType) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}

	c.workqueue.Add([2]string{key, event.String()})
}

// Run starts the controller.
func (c *Controller) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	logger := klog.FromContext(ctx)
	logger.V(1).Info("Starting controller")
	logger.V(4).Info("Waiting for informer caches to sync")

	// Start the informer factories to begin populating the informer caches.
	c.crsmInformerFactory.Start(ctx.Done())
	if ok := cache.WaitForCacheSync(ctx.Done(), c.crsmInformerFactory.Crsm().V1alpha1().CustomResourceStateMetricsResources().Informer().HasSynced); !ok {
		return stderrors.New("failed to wait for caches to sync")
	}

	// Build the telemetry registry.
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		versioncollector.NewCollector(version.ControllerName),
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{Namespace: version.ControllerName, ReportErrors: true}),
	)
	requestDurationVec := promauto.With(registry).NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "A histogram of requests for the main server's metrics endpoint.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "code"},
	)

	// Build servers.
	c.crsmUIDToStores = make(map[types.UID][]*StoreType)
	selfHost := *c.options.SelfHost
	selfPort := *c.options.SelfPort
	selfAddr := net.JoinHostPort(selfHost, strconv.Itoa(selfPort))
	logger.V(1).Info("Configuring self server", "address", selfAddr)
	selfInstance := newSelfServer(
		net.JoinHostPort(selfHost, strconv.Itoa(selfPort)),
	)
	self := selfInstance.build(ctx, c.kubeclientset, registry)
	mainHost := *c.options.MainHost
	mainPort := *c.options.MainPort
	mainAddr := net.JoinHostPort(mainHost, strconv.Itoa(mainPort))
	logger.V(1).Info("Configuring main server", "address", mainAddr)
	mainInstance := newMainServer(
		mainAddr,
		c.crsmUIDToStores,
		requestDurationVec,
	)
	main := mainInstance.build(ctx, c.kubeclientset, registry)

	// Launch `workers` amount of goroutines to process the work queue.
	logger.V(1).Info("Starting workers")
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, func(ctx context.Context) {

			// Nothing will be done if there are no enqueued items. Work-queues are thread-safe.
			for c.processNextWorkItem(ctx) {
			}
		}, time.Second)
	}

	// Start serving.
	go func() {
		logger.V(1).Info("Starting telemetry server")
		if err := self.ListenAndServe(); err != nil {
			logger.Error(err, "stopping telemetry server")
		}
	}()
	go func() {
		logger.V(1).Info("Starting main server")
		if err := main.ListenAndServe(); err != nil {
			logger.Error(err, "stopping main server")
		}
	}()

	// Stop serving on context cancellation.
	<-ctx.Done()
	logger.V(1).Info("Shutting down servers")
	err := self.Shutdown(ctx)
	if err != nil {
		logger.Error(err, "error shutting down telemetry server")
	}
	err = main.Shutdown(ctx)
	if err != nil {
		logger.Error(err, "error shutting down main server")
	}

	return nil
}

// processNextWorkItem retrieves each queued item and takes the necessary handler action, if the item has a valid object key.
// Whether the item itself is a valid object or not (tombstone), is checked further down the line.
func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	logger := klog.FromContext(ctx)

	// Retrieve the next item from the queue.
	objectWithEventInterface, shutdown := c.workqueue.Get()
	objectWithEvent := objectWithEventInterface.([2]string)
	if shutdown {
		return false
	}

	// Wrap this block in a func, so we can defer c.workqueue.Done. Forget the item if its invalid or processed.
	err := func(objectWithEvent [2]string) error {
		defer c.workqueue.Done(objectWithEvent)
		key := objectWithEvent[0]
		event := objectWithEvent[1]
		if err := c.syncHandler(ctx, key, event); err != nil {

			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(objectWithEvent)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}

		// Finally, if no error occurs we Forget this item, so it does not
		// get queued again until another change happens. Done has no effect
		// after Forget, so we must call it before.
		c.workqueue.Forget(objectWithEvent)
		logger.V(4).Info("Synced", "key", key)
		return nil // Do not requeue.
	}(objectWithEvent)
	if err != nil {
		logger.Error(err, "error processing item")
		return true
	}

	return true
}

// syncHandler resolves the object key, and sends it down for processing.
func (c *Controller) syncHandler(ctx context.Context, key string, event string) error {
	logger := klog.FromContext(ctx)
	logger.V(4).Info("Syncing", "key", key, "event", event)

	// Extract the namespace and name from the key.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logger.Error(err, "invalid resource key", "key", key)
		return nil // Do not requeue.
	}

	// Get the CRSMR resource with this namespace and name.
	resource, err := c.crsmInformerFactory.Crsm().V1alpha1().CustomResourceStateMetricsResources().Lister().
		CustomResourceStateMetricsResources(namespace).Get(name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("error getting CustomResourceStateMetricsResource %q: %w", klog.KRef(namespace, name), err)
		}

		resource = &v1alpha1.CustomResourceStateMetricsResource{}
		resource.SetName(name)
	}

	return c.handleObject(ctx, resource, event)
}

func (c *Controller) handleObject(ctx context.Context, objectI interface{}, event string) error {
	logger := klog.FromContext(ctx)

	// Check if the object is nil, and if so, handle it.
	if objectI == nil {
		logger.Error(stderrors.New("received nil object for handling, skipping"), "error handling object")

		// No point in re-queueing.
		return nil
	}

	// Check if the object is a valid tombstone, and if so, recover and process it.
	var (
		object metav1.Object
		ok     bool
	)
	if object, ok = objectI.(metav1.Object); !ok {
		tombstone, ok := objectI.(cache.DeletedFinalStateUnknown)
		if !ok {
			logger.Error(stderrors.New("error decoding object, invalid type"), "error handling object")

			// No point in re-queueing.
			return nil
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			logger.Error(stderrors.New("error decoding object tombstone, invalid type"), "error handling object")

			// No point in re-queueing.
			return nil
		}
		logger.V(1).Info("Recovered", "key", klog.KObj(object))
	}

	// Process the object based on its type.
	logger = klog.LoggerWithValues(klog.FromContext(ctx), "key", klog.KObj(object), "event", event)
	logger.V(1).Info("Processing object")
	switch o := object.(type) {
	case *v1alpha1.CustomResourceStateMetricsResource:
		handler := newCRSMHandler(c.kubeclientset, c.crsmClientset, c.dynamicClientset)
		return handler.handleEvent(ctx, c.crsmUIDToStores, event, o, *c.options.TryNoCache)
	default:
		logger.Error(stderrors.New("unknown object type"), "cannot handle object")
		return nil // Do not requeue.
	}
}
