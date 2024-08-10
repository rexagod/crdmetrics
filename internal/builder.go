package internal

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/gobuffalo/flect"
)

// buildStore builds a cache.Store for the metrics store.
func buildStore(
	ctx context.Context,
	dynamicClientset dynamic.Interface,
	expectedType interface{},
	familyGenerators []*familyGenerator,
	tryNoCache bool, // Retrieved from options.
	labelSelector, fieldSelector string, // Retrieved from the configuration.
) (*Store, error) {
	logger := klog.FromContext(ctx)

	// Create the reflector's LW.
	// NOTE: We generateFamily resource strings the same way as Kubebuilder (and KSM), using `flect`.
	apiVersionString := expectedType.(*unstructured.Unstructured).Object["apiVersion"].(string)
	expectedTypeSlice := strings.Split(apiVersionString, "/")
	g, v := expectedTypeSlice[0], expectedTypeSlice[1]
	if len(expectedTypeSlice) == 1 {
		g, v = "", expectedTypeSlice[0]
	}
	k := expectedType.(*unstructured.Unstructured).Object["kind"].(string)
	gvr := schema.GroupVersionResource{
		Group:    g,
		Version:  v,
		Resource: strings.ToLower(flect.Pluralize(k)),
	}
	lwo := metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
	}
	resourceVersionLatestBestEffort := "0"
	if tryNoCache {
		lwo.ResourceVersionMatch = metav1.ResourceVersionMatchNotOlderThan
		lwo.ResourceVersion = resourceVersionLatestBestEffort
	}
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options = lwo
			return dynamicClientset.Resource(gvr).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options = lwo
			return dynamicClientset.Resource(gvr).Watch(ctx, options)
		},
	}

	// Create the reflector's store.
	headers := make([]string, len(familyGenerators))
	for i, fg := range familyGenerators {
		headers[i] = fg.generateHeaderString()
	}
	metricFamiliesGenerator := func(object interface{}) []*family {
		families := make([]*family, len(familyGenerators))
		for i, fg := range familyGenerators {
			families[i] = fg.generateFamily(object)
		}
		return families
	}
	store := newStore(logger, headers, metricFamiliesGenerator)

	// Create and start the reflector.
	reflector := cache.NewReflectorWithOptions(lw, expectedType, store, cache.ReflectorOptions{
		Name: fmt.Sprintf("%#q reflector", gvr.String()),
		// TypeDescription is inferred from the *unstructured.Unstructured object's `apiVersion` and `kind` fields.
		ResyncPeriod: 0})
	go reflector.Run(ctx.Done())

	return store, nil
}
