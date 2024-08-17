package internal

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"github.com/rexagod/crsm/pkg/apis/crsm/v1alpha1"
)

// configure defines behaviours for working with configuration(s), can be implemented to use configurations other than
// the CEL one.
type configure interface {

	// Parse parses the given configuration.
	parse(string) error

	// build builds the given configuration.
	build(map[types.UID][]*StoreType, bool)
}

// configuration defines the structured representation of a CEL-based YAML configuration.
type configuration struct {
	Stores []*StoreType `yaml:"stores"`
}

// configurer knows how to parse a CEL-based YAML configuration.
type configurer struct {

	// ctx is the controller's context.
	ctx context.Context

	// configuration is the structured (parsed?) configuration.
	configuration configuration

	// dynamicClientset is the dynamic clientset used to build stores for different objects.
	dynamicClientset dynamic.Interface

	// resource is the resource to build stores for.
	resource *v1alpha1.CustomResourceStateMetricsResource
}

// configurer implements the configure interface.
var _ configure = &configurer{}

// newConfigurer returns a new configurer.
func newConfigurer(
	ctx context.Context,
	dynamicClientset dynamic.Interface,
	resource *v1alpha1.CustomResourceStateMetricsResource,
) *configurer {
	return &configurer{
		ctx:              ctx,
		dynamicClientset: dynamicClientset,
		resource:         resource,
	}
}

// parse knows how to parse the given configuration.
func (c *configurer) parse(configurationRaw string) error {
	err := yaml.Unmarshal([]byte(configurationRaw), &c.configuration)
	if err != nil {
		err = fmt.Errorf("error unmarshalling configuration: %w", err)
	}
	return err
}

// build knows how to build the given configuration.
func (c *configurer) build(crsmUIDToStoresMap map[types.UID][]*StoreType, tryNoCache bool) {
	for _, storeConfiguration := range c.configuration.Stores {
		g, v, k, r := storeConfiguration.Group, storeConfiguration.Version, storeConfiguration.Kind, storeConfiguration.ResourceName
		gvkWithR := gvkr{
			GroupVersionKind:     schema.GroupVersionKind{Group: g, Version: v, Kind: k},
			GroupVersionResource: schema.GroupVersionResource{Group: g, Version: v, Resource: r},
		}
		ls, fs := storeConfiguration.Selectors.Label, storeConfiguration.Selectors.Field
		families := storeConfiguration.Families
		resolver := storeConfiguration.Resolver
		labelKeys, labelValues := storeConfiguration.LabelKeys, storeConfiguration.LabelValues
		s := buildStore(
			c.ctx, c.dynamicClientset,
			gvkWithR,
			families,
			tryNoCache,
			ls, fs,
			resolver,
			labelKeys, labelValues,
		)
		resourceUID := c.resource.GetUID()
		crsmUIDToStoresMap[resourceUID] = append(crsmUIDToStoresMap[resourceUID], s)
	}
}
