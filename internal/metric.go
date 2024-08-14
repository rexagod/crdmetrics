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
}

// writeTo writes the given metric to the given strings.Builder.
func (m MetricType) writeTo(s *strings.Builder, g, v, k string) error {
	if len(m.LabelKeys) != len(m.resolvedLabelValues) {
		return fmt.Errorf("expected labelKeys %q to be of same length (%d) as labelValues %q (%d)", m.LabelKeys, len(m.LabelKeys), m.resolvedLabelValues, len(m.resolvedLabelValues))
	}

	// Copy labelset to avoid modifying the original (since these are evaluated on reconcile or reflector trigger).
	var labelKeys, labelValues []string
	copy(labelKeys, m.LabelKeys)
	copy(labelValues, m.LabelValues)

	// Append GVK metadata to the metric.
	labelKeys = append(labelKeys, "group", "version", "kind")
	labelValues = append(labelValues, g, v, k)

	// Write the metric.
	if len(labelKeys) > 0 {
		separator := "{"
		for i := 0; i < len(labelKeys); i++ {
			s.WriteString(separator)
			s.WriteString(labelKeys[i])
			s.WriteString("=\"")
			n, err := strings.NewReplacer("\\", `\\`, "\n", `\n`, "\"", `\"`).WriteString(s, labelValues[i])
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
