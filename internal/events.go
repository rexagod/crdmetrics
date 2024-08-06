package internal

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/rexagod/crsm/pkg/apis/crsm/v1alpha1"
	clientset "github.com/rexagod/crsm/pkg/generated/clientset/versioned"
)

type Event int

const (
	AddEvent Event = iota
	UpdateEvent
	DeleteEvent
)

func (e Event) String() string {
	return []string{"AddEvent", "UpdateEvent", "DeleteEvent"}[e]
}

// crsmEventHandler implements the eventHandler interface.
var _ eventHandler = &crsmEventHandler{}

// eventHandler knows how to handle informer events.
type eventHandler interface {

	// HandleEvent handles events received from the informer.
	HandleEvent(ctx context.Context, o metav1.Object, event string) error
}

// crsmEventHandler implements the EventHandler interface.
type crsmEventHandler struct {

	// namespace is the namespace of the crsm resource.
	namespace string

	// clientset is the clientset used to update the status of the crsm resource.
	clientset clientset.Interface
}

// HandleEvent handles events received from the informer.
func (h *crsmEventHandler) HandleEvent(ctx context.Context, o metav1.Object, event string) error {
	logger := klog.LoggerWithValues(klog.FromContext(ctx), "key", klog.KObj(o), "event", event)

	resource, ok := o.(*v1alpha1.CustomResourceStateMetricsResource)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("failed to cast object to %s", resource.GetObjectKind()))
		return nil // Do not requeue.
	}
	key, err := cache.MetaNamespaceKeyFunc(resource)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to get key for %s %s/%s: %w", resource.GetObjectKind(), resource.GetNamespace(), resource.GetName(), err))
		return nil // Do not requeue.
	}

	// Handle the event.
	switch event {

	// Generate metrics.
	case AddEvent.String():
		logger.V(4).Info("foo")

	// Refresh metrics.
	case UpdateEvent.String():
		logger.V(4).Info("bar")

	// Drop metrics.
	case DeleteEvent.String():
		logger.V(4).Info("baz")

	default:
		utilruntime.HandleError(fmt.Errorf("unknown event type (%s) for %s", event, key))
	}

	return nil
}
