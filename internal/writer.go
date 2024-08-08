package internal

import (
	"fmt"
	"io"
)

// metricsWriter knows how to write metrics for the groups of metric families present in the group of stores it holds
// to an io.Writer.
type metricsWriter struct {
	stores []*Store
}

// newMetricsWriter returns a new metricsWriter.
func newMetricsWriter(stores ...*Store) *metricsWriter {
	return &metricsWriter{
		stores: stores,
	}
}

// writeAllTo writes out metrics from the underlying stores to the given writer. It writes metrics so that the ones with
// the same name are grouped together when written out, and guarantees an exposition format that is safe to be ingested
// by Prometheus.
func (m metricsWriter) writeAllTo(w io.Writer) error {
	if len(m.stores) == 0 {
		return nil
	}
	for _, s := range m.stores {
		s.mutex.RLock()
		defer s.mutex.RUnlock()
	}
	for j := 0; j < len(m.stores); j++ {
		for i, help := range m.stores[j].headers {
			if help != "" && help != "\n" {
				help += "\n"
			}
			if len(m.stores[j].metrics) > 0 {
				n, err := w.Write([]byte(help))
				if err != nil {
					return fmt.Errorf("error writing help text (%s) after %d bytes: %w", help, n, err)
				}
			}
			for _, s := range m.stores {
				for _, metricFamilies := range s.metrics {
					n, err := w.Write(metricFamilies[i])
					if err != nil {
						return fmt.Errorf("error writing metric family after %d bytes: %w", n, err)
					}
				}
			}
		}
	}

	return nil
}
