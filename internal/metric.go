package internal

import (
	"fmt"
	"strconv"
	"strings"
)

// MetricType represents a single time series.
type MetricType struct {

	// LabelKeys is the set of label keys.
	LabelKeys []string `yaml:"labelKeys"`

	// LabelValues is the set of label values.
	LabelValues         []string `yaml:"labelValues"`
	resolvedLabelValues []string

	// Value is the metric Value.
	Value         string `yaml:"value"`
	resolvedValue string

	// Resolver is the resolver to use to evaluate the labelset expressions.
	Resolver ResolverType `yaml:"resolver"`
}

// writeTo writes the given metric to the given strings.Builder.
func (m MetricType) writeTo(s *strings.Builder, g, v, k string) error {
	if len(m.LabelKeys) != len(m.resolvedLabelValues) {
		return fmt.Errorf("expected labelKeys %q to be of same length (%d) as the resolved labelValues %q (%d)", m.LabelKeys, len(m.LabelKeys), m.resolvedLabelValues, len(m.resolvedLabelValues))
	}

	// Copy labelset to avoid modifying the original.
	labelKeys := make([]string, len(m.LabelKeys))
	resolvedLabelValues := make([]string, len(m.resolvedLabelValues))
	copy(labelKeys, m.LabelKeys)
	copy(resolvedLabelValues, m.resolvedLabelValues)

	// Append GVK metadata to the metric.
	labelKeys = append(labelKeys, "group", "version", "kind")
	resolvedLabelValues = append(resolvedLabelValues, g, v, k)

	// Write the metric.
	if len(labelKeys) > 0 {
		separator := "{"
		for i := 0; i < len(labelKeys); i++ {
			s.WriteString(separator)
			s.WriteString(labelKeys[i])
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
	metricValueAsFloat, err := strconv.ParseFloat(m.resolvedValue, 64)
	if err != nil {
		return fmt.Errorf("error parsing metric value %q as float64: %w", m.resolvedValue, err)
	}
	n, err := fmt.Fprintf(s, "%f", metricValueAsFloat)
	if err != nil {
		return fmt.Errorf("error writing (float64) metric value after %d bytes: %w", n, err)
	}
	s.WriteByte('\n')

	return nil
}
