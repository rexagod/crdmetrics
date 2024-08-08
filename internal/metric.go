package internal

import (
	"fmt"
	"strings"
)

// metric represents a single time series.
type metric struct {

	// Keys is the set of label keys.
	Keys []string

	// Values is the set of label values.
	Values []string

	// Value is the metric value.
	Value float64
}

// writeTo writes the given metric to the given strings.Builder.
func (m metric) writeTo(s *strings.Builder) error {
	if len(m.Keys) != len(m.Values) {
		return fmt.Errorf("expected labelKeys %q to be of same length (%d) as labelValues %q (%d)", m.Keys, len(m.Keys), m.Values, len(m.Values))
	}
	if len(m.Keys) > 0 {
		var separator byte = '{'
		for i := 0; i < len(m.Keys); i++ {
			s.WriteByte(separator)
			s.WriteString(m.Keys[i])
			s.WriteString("=\"")
			n, err := strings.NewReplacer("\\", `\\`, "\n", `\n`, "\"", `\"`).WriteString(s, m.Values[i])
			if err != nil {
				return fmt.Errorf("error writing metric after %d bytes: %w", n, err)
			}
			s.WriteByte('"')
			separator = ','
		}
		s.WriteByte('}')
	}
	s.WriteByte(' ')
	n, err := fmt.Fprintf(s, "%f", m.Value)
	if err != nil {
		return fmt.Errorf("error writing (float64) metric value after %d bytes: %w", n, err)
	}
	s.WriteByte('\n')

	return nil
}
