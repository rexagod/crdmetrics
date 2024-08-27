package framework

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// Runner provides relevant APIs needed to run tests.
type Runner struct {
	http.Client
}

// NewRunner returns a new Runner.
func NewRunner() *Runner {
	return &Runner{
		Client: http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: time.Second,
			},
		},
	}
}

// GetRaw writes the GetRaw response from the provided endpoint to the provided writer.
func (r *Runner) GetRaw(endpoint *url.URL) (string, error) {
	resp, err := r.Do(&http.Request{
		Method: http.MethodGet,
		URL:    endpoint,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get response: %w", err)
	}
	defer func() {
		_, err = io.Copy(io.Discard, resp.Body)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to discard response: %w", err))
		}
		err = resp.Body.Close()
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to close response body: %w", err))
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status %s", resp.Status)
	}
	raw := new(strings.Builder)
	_, err = io.Copy(raw, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to write response: %w", err)
	}

	return raw.String(), nil
}

// GetMetrics parses the textual response from the provided endpoint.
func (r *Runner) GetMetrics(endpoint *url.URL) (map[string]*dto.MetricFamily, error) {
	raw, err := r.GetRaw(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	textualParser := new(expfmt.TextParser)
	familyMap, err := textualParser.TextToMetricFamilies(strings.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("failed to parse metrics: %w", err)
	}

	return familyMap, nil
}
