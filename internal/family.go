/*
Copyright 2024 The Kubernetes crdmetrics Authors.

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
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	"github.com/rexagod/crdmetrics/pkg/resolver"
)

const (

	// metricTypeGauge represents the type of metric. This is pinned to `gauge` to avoid ingestion issues with different backends
	// (Prometheus primarily) that may not recognize all metrics under the OpenMetrics spec. This also helps upkeep a more
	// consistent configuration. Refer https://github.com/kubernetes/kube-state-metrics/pull/2270 for more details.
	metricTypeGauge = "gauge"

	// In convention with kube-state-metrics, we prefix all metrics with `kube_customresource_` to explicitly denote
	// that these are custom resource user-generated metrics (and have no stability).
	kubeCustomResourcePrefix = "kube_customresource_"
)

// ResolverType represents the type of resolver to use to evaluate the labelset expressions.
type ResolverType string

const (

	// ResolverTypeCEL represents the CEL resolver.
	ResolverTypeCEL ResolverType = "cel"

	// ResolverTypeUnstructured represents the Unstructured resolver.
	ResolverTypeUnstructured ResolverType = "unstructured"

	// ResolverTypeNone represents an empty resolver.
	ResolverTypeNone ResolverType = ""
)

// FamilyType represents a metric family (a group of metrics with the same name).
type FamilyType struct {

	// logger is the family's logger.
	logger klog.Logger

	// Name is the Name of the metric family.
	Name string `yaml:"name"`

	// Help is the Help text for the metric family.
	Help string `yaml:"help"`

	// t is the type of the metric family.
	// NOTE: This will always be pinned to `gauge`, and thus not exported for unmarshalling.
	t string

	// Metrics is a slice of Metrics that belong to the MetricType family.
	Metrics []*MetricType `yaml:"metrics"`

	// Resolver is the resolver to use to evaluate the labelset expressions.
	Resolver ResolverType `yaml:"resolver"`

	// LabelKeys is the set of inherited or defined label keys.
	LabelKeys []string `yaml:"labelKeys,omitempty"`

	// LabelValues is the set of inherited or defined label values.
	LabelValues []string `yaml:"labelValues,omitempty"`
}

// rawWith returns the given family in its byte representation.
func (f *FamilyType) rawWith(u *unstructured.Unstructured) string {
	logger := f.logger.WithValues("family", f.Name)

	s := strings.Builder{}
	for _, m := range f.Metrics {
		ss := strings.Builder{}

		// Inherit the label keys and values.
		m.LabelKeys = append(m.LabelKeys, f.LabelKeys...)
		m.LabelValues = append(m.LabelValues, f.LabelValues...)

		// Inherit the resolver.
		var resolverInstance resolver.Resolver
		if m.Resolver == ResolverTypeNone {
			m.Resolver = f.Resolver
		}
		switch m.Resolver {
		case ResolverTypeNone:
			fallthrough
		case ResolverTypeCEL:
			resolverInstance = resolver.NewCELResolver(logger)
		case ResolverTypeUnstructured:
			resolverInstance = resolver.NewUnstructuredResolver(logger)
		default:
			logger.V(1).Error(fmt.Errorf("error resolving metric: unknown resolver %q", m.Resolver), "skipping")
			continue
		}

		// Resolve the labelset.
		var (
			resolvedLabelKeys   []string
			resolvedLabelValues []string
		)
		for i, query := range m.LabelValues {
			resolvedLabelset := resolverInstance.Resolve(query, u.Object)

			// If the query is found in the resolved labelset, append the resolved value.
			if resolvedLabelValue, ok := resolvedLabelset[query]; ok {
				resolvedLabelValues = append(resolvedLabelValues, resolvedLabelValue)

				// Label keys are not resolved if the returned labelset for the same label key exists.
				resolvedLabelKeys = append(resolvedLabelKeys, strings.ToLower(regexp.MustCompile(`\W`).
					ReplaceAllString(m.LabelKeys[i], "_")))

				// If the query is not found in the resolved labelset, it is now redundant as a label value.
			} else {
				for k, v := range resolvedLabelset {
					resolvedLabelValues = append(resolvedLabelValues, v)

					// Label keys are resolved (with the original label keys being the new label key's prefix) if the
					// returned labelset for the same label key does not exist.
					resolvedLabelKeys = append(resolvedLabelKeys, strings.ToLower(regexp.MustCompile(`\W`).
						ReplaceAllString(m.LabelKeys[i]+k, "_")))
				}
			}
		}

		// Resolve the metric value.
		resolvedValue, found := resolverInstance.Resolve(m.Value, u.Object)[m.Value]
		if !found {
			logger.V(1).Error(fmt.Errorf("error resolving metric value %q", m.Value), "skipping")
			continue
		}

		// Write the metric.
		ss.WriteString(kubeCustomResourcePrefix)
		ss.WriteString(f.Name)
		err := writeMetricTo(
			&ss,
			u.GroupVersionKind().Group, u.GroupVersionKind().Version, u.GroupVersionKind().Kind,
			resolvedValue,
			resolvedLabelKeys, resolvedLabelValues,
		)
		if err != nil {
			logger.V(1).Error(fmt.Errorf("error writing metric: %w", err), "skipping")
			continue
		}

		s.WriteString(ss.String())
	}

	return s.String()
}

// buildHeaders generates the header for the given family.
func (f *FamilyType) buildHeaders() string {
	header := strings.Builder{}

	// Write the help text.
	header.WriteString("# HELP ")
	header.WriteString(kubeCustomResourcePrefix)
	header.WriteString(f.Name)
	header.WriteString(" ")
	header.WriteString(f.Help)
	header.WriteString("\n")

	// Write the type text.
	header.WriteString("# TYPE ")
	header.WriteString(kubeCustomResourcePrefix)
	header.WriteString(f.Name)
	header.WriteString(" ")
	f.t = metricTypeGauge
	header.WriteString(f.t)

	return header.String()
}
