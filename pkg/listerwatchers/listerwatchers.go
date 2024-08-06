package listerwatchers

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
)

// DynamicListerWatcher knows how to generate listers and watchers.
type DynamicListerWatcher struct {
	*dynamic.DynamicClient
}

// metaToSchemaGVR converts a metav1.GroupVersionResource into schema.GroupVersionResource.
// We cannot depend on the latter for code-gen (CRD types) since that has no tags defined.
func metaToSchemaGVR(gvr metav1.GroupVersionResource) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    gvr.Group,
		Version:  gvr.Version,
		Resource: gvr.Resource,
	}
}

// GenerateLWForGVK returns a *cache.ListWatch implementation that can be used to set up SharedIndexInformers for the provided GVR.
func (dlw *DynamicListerWatcher) GenerateLWForGVK(ctx context.Context, gvr metav1.GroupVersionResource, listOptions metav1.ListOptions) *cache.ListWatch {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return dlw.DynamicClient.Resource(metaToSchemaGVR(gvr)).List(ctx, listOptions)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return dlw.DynamicClient.Resource(metaToSchemaGVR(gvr)).Watch(ctx, listOptions)
		},
	}
}
