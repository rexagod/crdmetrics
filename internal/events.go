package internal

import (
	"context"
	"fmt"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/rexagod/crsm/pkg/apis/crsm/v1alpha1"
	clientset "github.com/rexagod/crsm/pkg/generated/clientset/versioned"
)

type eventType int

const (
	AddEvent eventType = iota
	UpdateEvent
	DeleteEvent
)

func (e eventType) String() string {
	return []string{"AddEvent", "UpdateEvent", "DeleteEvent"}[e]
}

// crsmEventHandler knows how to handle CRSM events.
type crsmEventHandler struct {

	// namespace is the namespace of the crsm resource.
	namespace string

	// clientset is the clientset used to update the status of the crsm resource.
	clientset clientset.Interface

	// crsmrStoresMap is the handler's internal cache of crsmr objects mapped to the stores they are linked to.
	// When a CRSMR is added or updated, the cache will be populated with that object's stores. Similarly,
	// when a CRSMR is deleted, the cache will be purged of that object's stores.
	crsmrStoresMap map[types.UID][]Store
}

// HandleEvent handles events received from the informer.
func (h *crsmEventHandler) handleEvent(ctx context.Context, dynamicClientset dynamic.Interface, o metav1.Object, event string) error {
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

	// Refresh metrics.
	case AddEvent.String(), UpdateEvent.String():
		store, err := buildStore(
			ctx,
			dynamicClientset,
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "contoso.com/v1alpha1",
					"kind":       "MyPlatform",
				},
			},
			[]*familyGenerator{
				newFamilyGenerator(
					"foo_metric",
					"foo_help",
					"gauge",
					func(i interface{}) *family {
						iA, err := meta.Accessor(i)
						if err != nil {
							fmt.Println(err)
							return nil
						}
						return &family{
							metrics: []*metric{
								{
									Keys:   []string{"foo"},
									Values: []string{iA.GetName()},
									Value:  2.0,
								},
							},
						}
					})},
			false, "", "")
		if err != nil {
			return err
		}
		time.Sleep(5 * time.Second)
		newMetricsWriter(store).writeAllTo(os.Stdout)

	// Drop metrics.
	case DeleteEvent.String():
		logger.V(4).Info("baz")

	default:
		utilruntime.HandleError(fmt.Errorf("unknown event type (%s) for %s", event, key))
	}

	return nil
}
