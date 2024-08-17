package internal

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// MetricType represents a single time series.
type MetricType struct {

	// LabelKeys is the set of label keys.
	LabelKeys []string `yaml:"labelKeys"`

	// LabelValues is the set of label values.
	LabelValues []string `yaml:"labelValues"`

	// Value is the metric Value.
	Value string `yaml:"value"`

	// Resolver is the resolver to use to evaluate the labelset expressions.
	Resolver ResolverType `yaml:"resolver"`
}

// writeMetricTo writes the given metric to the given strings.Builder.
func writeMetricTo(s *strings.Builder, g, v, k, resolvedValue string, resolvedLabelKeys, resolvedLabelValues []string) error {
	if len(resolvedLabelKeys) != len(resolvedLabelValues) {
		return fmt.Errorf("expected labelKeys %q to be of same length (%d) as the resolved labelValues %q (%d)", resolvedLabelKeys, len(resolvedLabelKeys), resolvedLabelValues, len(resolvedLabelValues))
	}

	// Sort the label keys and values. This preserves order and helps test deterministically.
	sortLabelset(resolvedLabelKeys, resolvedLabelValues)

	// Append GVK metadata to the metric.
	resolvedLabelKeys = append(resolvedLabelKeys, "group", "version", "kind")
	resolvedLabelValues = append(resolvedLabelValues, g, v, k)

	// Write the metric.
	if len(resolvedLabelKeys) > 0 {
		separator := "{"
		for i := 0; i < len(resolvedLabelKeys); i++ {
			s.WriteString(separator)
			s.WriteString(resolvedLabelKeys[i])
			s.WriteString("=\"")
			n, err := strings.NewReplacer("\\", `\\`, "\n", `\n`, "\"", `\"`).WriteString(s, resolvedLabelValues[i])
			if err != nil {
				return fmt.Errorf("error writing metric after %d bytes: %w", n, err)
			}
			s.WriteString("\"")
			separator = ","
		}
		s.WriteString("}")
	}
	s.WriteByte(' ')
	metricValueAsFloat, err := strconv.ParseFloat(resolvedValue, 64)
	if err != nil {
		return fmt.Errorf("error parsing metric value %q as float64: %w", resolvedValue, err)
	}
	n, err := fmt.Fprintf(s, "%f", metricValueAsFloat)
	if err != nil {
		return fmt.Errorf("error writing (float64) metric value after %d bytes: %w", n, err)
	}
	s.WriteByte('\n')

	return nil
}

// sortLabelset sorts the label keys and values while preserving order.
func sortLabelset(resolvedLabelKeys, resolvedLabelValues []string) {

	// Populate.
	type labelset struct {
		labelKey   string
		labelValue string
	}
	labelsets := make([]labelset, len(resolvedLabelKeys))
	for i := range resolvedLabelKeys {
		labelsets[i] = labelset{labelKey: resolvedLabelKeys[i], labelValue: resolvedLabelValues[i]}
	}

	// Sort.
	sort.Slice(labelsets, func(i, j int) bool {
		a, b := labelsets[i].labelKey, labelsets[j].labelKey
		if len(a) == len(b) {
			return a < b
		}
		return len(a) < len(b)
	})

	// Re-populate.
	for i := range labelsets {
		resolvedLabelKeys[i] = labelsets[i].labelKey
		resolvedLabelValues[i] = labelsets[i].labelValue
	}
}
