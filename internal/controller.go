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

package internal

import (
	"context"
	"fmt"
	"os"
	"time"

	"golang.org/x/time/rate"
	"k8s.io/client-go/dynamic"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/davecgh/go-spew/spew"
	"github.com/rexagod/crsm/pkg/apis/crsm/v1alpha1"
	clientset "github.com/rexagod/crsm/pkg/generated/clientset/versioned"
	crsmscheme "github.com/rexagod/crsm/pkg/generated/clientset/versioned/scheme"
	informers "github.com/rexagod/crsm/pkg/generated/informers/externalversions"
)

// controllerName is the event source for the recorder.
const controllerName = "crsm-controller"

// Controller is the controller implementation for CustomResourceStateMetricsResource resources.
type Controller struct {

	// kubeclientset is a standard kubernetes clientset, required for native operations.
	kubeclientset kubernetes.Interface

	// crsmClientset is a clientset for our own API group.
	crsmClientset clientset.Interface

	// dynamicClientset is a clientset for CRD operations.
	dynamicClientset dynamic.Interface

	// crsmInformerFactory is a shared informer factory for crsm resources.
	crsmInformerFactory informers.SharedInformerFactory

	// workqueue is a rate limited work queue. This is used to queue work to be processed instead of performing it as
	// soon as a change happens. This means we can ensure we only process a fixed amount of resources at a time, and
	// makes it easy to ensure we are never processing the same item simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface

	// recorder is an event recorder for recording event resources.
	recorder record.EventRecorder
}

// NewController returns a new sample controller.
func NewController(ctx context.Context, kubeClientset kubernetes.Interface, crsmClientset clientset.Interface, dynamicClientset dynamic.Interface) *Controller {

	// Add native resources to the default Kubernetes Scheme so Events can be logged for them.
	utilruntime.Must(crsmscheme.AddToScheme(scheme.Scheme))

	// Initialize the controller.
	logger := klog.FromContext(ctx)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClientset.CoreV1().Events(os.Getenv("POD_NAMESPACE") /* emit in the default namespace if none is defined */)})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerName})
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
	}

	// Set up event handlers for CustomResourceStateMetricsResource resources.
	logger.V(4).Info("Setting up event handlers")
	_, err := controller.crsmInformerFactory.Crsm().V1alpha1().CustomResourceStateMetricsResources().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			controller.enqueueCRSMResource(obj, AddEvent)
		},
		UpdateFunc: func(old, new interface{}) {
			if old.(*v1alpha1.CustomResourceStateMetricsResource).ResourceVersion == new.(*v1alpha1.CustomResourceStateMetricsResource).ResourceVersion {
				return
			}
			controller.enqueueCRSMResource(new, UpdateEvent)
		},
		DeleteFunc: func(obj interface{}) {
			controller.enqueueCRSMResource(obj, DeleteEvent)
		},
	})
	if err != nil {
		klog.Fatal(err)
	}

	return controller
}

// enqueueCRSMResource takes a CustomResourceStateMetricsResource resource and converts it into a namespace/name key.
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
	logger.V(4).Info("Starting CustomResourceStateMetricsResource controller")
	logger.V(4).Info("Waiting for informer caches to sync")

	// Start the informer factories to begin populating the informer caches.
	c.crsmInformerFactory.Start(ctx.Done())
	if ok := cache.WaitForCacheSync(ctx.Done(), c.crsmInformerFactory.Crsm().V1alpha1().CustomResourceStateMetricsResources().Informer().HasSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	// Launch `workers` amount of goroutines to process the work queue.
	logger.Info("Starting workers", "count", workers)
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, func(ctx context.Context) {

			// Run every second. Nothing will be done if there are no enqueued items. Work-queues are thread-safe.
			for c.processNextWorkItem(ctx) {
			}
		}, time.Second)
	}

	logger.V(4).Info("Started workers", "count", workers)
	<-ctx.Done()
	logger.V(4).Info("Shutting down workers", "count", workers)

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
		logger.V(1).Info("Synced", "key", key)
		return nil
	}(objectWithEvent)
	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler resolves the object key, and sends it down for processing.
func (c *Controller) syncHandler(ctx context.Context, key string, event string) error {
	logger := klog.FromContext(ctx)
	logger.V(1).Info("Syncing", "key", key, "event", event)

	// Extract the namespace and name from the key.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the CustomResourceStateMetricsResource resource with this namespace and name.
	crsmResource, err := c.crsmInformerFactory.Crsm().V1alpha1().CustomResourceStateMetricsResources().Lister().CustomResourceStateMetricsResources(namespace).Get(name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("error getting CustomResourceStateMetricsResource '%s/%s': %w", namespace, name, err)
		}

		crsmResource = &v1alpha1.CustomResourceStateMetricsResource{}
		crsmResource.SetName(name)
	}

	return c.handleObject(ctx, crsmResource, event)
}

func (c *Controller) handleObject(ctx context.Context, obj interface{}, event string) error {
	logger := klog.FromContext(ctx)

	// Check if the object is nil, and if so, handle it.
	if obj == nil {
		utilruntime.HandleError(fmt.Errorf("recieved nil object for handling, skipping"))

		// No point in re-queueing.
		return nil
	}

	// Check if the object is a valid tombstone, and if so, recover and process it.
	var (
		object metav1.Object
		ok     bool
	)
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))

			// No point in re-queueing.
			return nil
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))

			// No point in re-queueing.
			return nil
		}
		logger.V(1).Info("Recovered", "key", "key", klog.KObj(object))
	}

	// Process the object based on its type.
	logger = klog.LoggerWithValues(klog.FromContext(ctx), "key", klog.KObj(object), "event", event)
	logger.V(1).Info("Processing")
	switch o := object.(type) {
	case *v1alpha1.CustomResourceStateMetricsResource:
		handler := &crsmEventHandler{
			namespace: object.GetNamespace(),
			clientset: c.crsmClientset,
		}
		return handler.handleEvent(ctx, c.dynamicClientset, o, event)
	default:
		utilruntime.HandleError(fmt.Errorf("unknown object type: %T, full schema below:\n%s", o, spew.Sdump(obj)))
	}

	return nil
}
