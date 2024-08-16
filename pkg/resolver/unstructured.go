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
func NewUnstructuredResolver() *UnstructuredResolver {
	return &UnstructuredResolver{}
}

// Resolve resolves the given query against the given unstructured object.
func (ur *UnstructuredResolver) Resolve(query string, unstructuredObjectMap map[string]interface{}) string {
	ur.logger = ur.logger.WithValues("query", query)

	resolvedI, found, err := unstructured.NestedFieldNoCopy(unstructuredObjectMap, strings.Split(query, ".")...)
	if !found {
		return query
	}
	if err != nil {
		ur.logger.Info("ignoring resolution for query")
		return query
	}

	return fmt.Sprintf("%v", resolvedI)
}
