package internal

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"

	"github.com/rexagod/crsm/pkg/apis/crsm/v1alpha1"
	clientset "github.com/rexagod/crsm/pkg/generated/clientset/versioned"
)

// eventType represents the type of event received from the informer.
type eventType int

const (
	AddEvent eventType = iota
	UpdateEvent
	DeleteEvent
)

func (e eventType) String() string {
	return []string{"AddEvent", "UpdateEvent", "DeleteEvent"}[e]
}

// crsmHandler knows how to handle CRSM events.
type crsmHandler struct {

	// clientset is the clientset used to update the status of the CRSM resource.
	clientset clientset.Interface

	// dynamicClientset is the dynamic clientset used to build stores for different objects.
	dynamicClientset dynamic.Interface
}

// newCRSMHandler creates a new crsmHandler.
func newCRSMHandler(clientset clientset.Interface, dynamicClientset dynamic.Interface) *crsmHandler {
	return &crsmHandler{
		clientset:        clientset,
		dynamicClientset: dynamicClientset,
	}
}

// HandleEvent handles events received from the informer.
func (h *crsmHandler) handleEvent(ctx context.Context, crsmUIDToStoresMap map[types.UID][]*Store, event string, o metav1.Object) error {

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

	// Build all associated stores.
	case AddEvent.String(), UpdateEvent.String():
		// TODO: Once the CEL configuration support is added, the family generators should be created based on the it.
		// The snippet below is purely for development purposes.
		store, _ := buildStore(ctx, h.dynamicClientset, &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "contoso.com/v1alpha1",
			"kind":       "MyPlatform"}},
			[]*familyGenerator{newFamilyGenerator("foo_metric", "foo_help", "gauge",
				func(i interface{} /* *unstructured.Unstructured */) *family {
					iA, err := meta.Accessor(i)
					if err != nil {
						return nil
					}
					return &family{metrics: []*metric{{Keys: []string{"foo"}, Values: []string{iA.GetName()}, Value: 2.0}}}
				})}, false, "", "")

		resourceUID := resource.GetUID()
		store.crsmrUID = resourceUID
		crsmUIDToStoresMap[resourceUID] = append(crsmUIDToStoresMap[resourceUID], store)

	// Drop all associated stores.
	case DeleteEvent.String():
		crsmrUID := resource.GetUID()
		if _, ok := crsmUIDToStoresMap[crsmrUID]; ok {
			// The associated stores are only reachable through the map. Deleting them will trigger the GC.
			delete(crsmUIDToStoresMap, crsmrUID)
		}

	default:
		utilruntime.HandleError(fmt.Errorf("unknown event type (%s) for %s", event, key))
	}

	return nil
}
