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
	LabelValues []string `yaml:"labelValues"`

	// Value is the metric Value.
	Value string `yaml:"value"`
}

// writeTo writes the given metric to the given strings.Builder.
func (m MetricType) writeTo(s *strings.Builder) error {
	if len(m.LabelKeys) != len(m.LabelValues) {
		return fmt.Errorf("expected labelKeys %q to be of same length (%d) as labelValues %q (%d)", m.LabelKeys, len(m.LabelKeys), m.LabelValues, len(m.LabelValues))
	}
	if len(m.LabelKeys) > 0 {
		var separator byte = '{'
		for i := 0; i < len(m.LabelKeys); i++ {
			s.WriteByte(separator)
			s.WriteString(m.LabelKeys[i])
			s.WriteString("=\"")
			n, err := strings.NewReplacer("\\", `\\`, "\n", `\n`, "\"", `\"`).WriteString(s, m.LabelValues[i])
			if err != nil {
				return fmt.Errorf("error writing metric after %d bytes: %w", n, err)
			}
			s.WriteByte('"')
			separator = ','
		}
		s.WriteByte('}')
	}
	s.WriteByte(' ')
	metricValueAsFloat, err := strconv.ParseFloat(m.Value, 64)
	if err != nil {
		return fmt.Errorf("error parsing metric value %q as float64: %w", m.Value, err)
	}
	n, err := fmt.Fprintf(s, "%f", metricValueAsFloat)
	if err != nil {
		return fmt.Errorf("error writing (float64) metric value after %d bytes: %w", n, err)
	}
	s.WriteByte('\n')

	return nil
}
