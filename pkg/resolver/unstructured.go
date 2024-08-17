package resolver

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
)

// UnstructuredResolver represents a resolver for unstructured objects.
type UnstructuredResolver struct {
	logger klog.Logger
}

// UnstructuredResolver implements the Resolver interface.
var _ Resolver = &UnstructuredResolver{}

// NewUnstructuredResolver returns a new unstructured resolver.
func NewUnstructuredResolver(logger klog.Logger) *UnstructuredResolver {
	return &UnstructuredResolver{logger: logger}
}

// Resolve resolves the given query against the given unstructured object.
// NOTE: Resolutions resulting in composite values for label keys and values are not supported, owing to upstream
// limitations: https://github.com/kubernetes/apimachinery/blob/v0.31.0/pkg/apis/meta/v1/unstructured/helpers_test.go#L121.
func (ur *UnstructuredResolver) Resolve(query string, unstructuredObjectMap map[string]interface{}) map[string]string {
	ur.logger = ur.logger.WithValues("query", query)

	resolvedI, found, err := unstructured.NestedFieldNoCopy(unstructuredObjectMap, strings.Split(query, ".")...)
	if !found {
		return map[string]string{query: query}
	}
	if err != nil {
		ur.logger.V(1).Info("ignoring resolution for query", "info", err)
		return map[string]string{query: query}
	}

	return map[string]string{query: fmt.Sprintf("%v", resolvedI)}
}
