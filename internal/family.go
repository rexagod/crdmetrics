package internal

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

// FamilyType represents a metric family (a group of metrics with the same name).
type FamilyType struct {

	// Name is the Name of the metric family.
	Name string `yaml:"name"`

	// Help is the Help text for the metric family.
	Help string `yaml:"help"`

	// t is the type of the metric family.
	// NOTE: This will always be pinned to `gauge`, and thus not exported for unmarshalling.
	t string

	// Metrics is a slice of Metrics that belong to the MetricType family.
	Metrics []*MetricType `yaml:"metrics"`
}

// rawWith returns the given family in its byte representation.
func (f *FamilyType) rawWith(u *unstructured.Unstructured) (string, error) {
	s := strings.Builder{}
	for _, m := range f.Metrics {

		// Resolve the label values.
		resolvedLabelValues := make([]string, 0, len(m.LabelValues))
		for _, query := range m.LabelValues {
			resolvedI, _, err := unstructured.NestedFieldNoCopy(u.Object, strings.Split(query, ".")...)
			if err != nil {
				return "", fmt.Errorf("error resolving %s: %w", query, err)
			}
			resolvedLabelValues = append(resolvedLabelValues, fmt.Sprintf("%v", resolvedI))
		}
		m.LabelValues = resolvedLabelValues

		// Append GVK to the labelset.
		m.LabelKeys = append(m.LabelKeys, "group", "version", "kind")
		m.LabelValues = append(m.LabelValues, u.GroupVersionKind().Group, u.GroupVersionKind().Version, u.GroupVersionKind().Kind)

		// Resolve the metric value.
		var resolvedValue interface{}
		var err error

		// Check if the metric value string is a float64.
		resolvedValue, err = strconv.ParseFloat(m.Value, 64)
		if err != nil {

			// Try to resolve the metric value otherwise.
			resolvedValue, _, err = unstructured.NestedFieldNoCopy(u.Object, strings.Split(m.Value, ".")...)
			if err != nil {
				return "", fmt.Errorf("error resolving %s: %w", m.Value, err)
			}
		}
		m.Value = fmt.Sprintf("%v", resolvedValue)

		// Write the metric.
		s.WriteString(kubeCustomResourcePrefix)
		s.WriteString(f.Name)
		err = m.writeTo(&s)
		if err != nil {
			return "", fmt.Errorf("error writing %s metric: %w", f.Name, err)
		}
	}

	return s.String(), nil
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
