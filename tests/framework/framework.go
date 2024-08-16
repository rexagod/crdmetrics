package framework

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

// Runner provides relevant APIs needed to run tests.
type Runner struct {
	context.Context
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
	resp, err := r.Get(endpoint.String())
	if err != nil {
		return "", err
	}
	defer func() {
		_, err = io.Copy(io.Discard, resp.Body)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to discard response: %v", err))
		}
		err = resp.Body.Close()
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to close response body: %v", err))
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status %s", resp.Status)
	}
	raw := new(strings.Builder)
	_, err = io.Copy(raw, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to write response: %v", err)
	}

	return raw.String(), nil
}

// GetMetrics parses the textual response from the provided endpoint.
func (r *Runner) GetMetrics(endpoint *url.URL) (map[string]*dto.MetricFamily, error) {
	raw, err := r.GetRaw(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %v", err)
	}

	textualParser := new(expfmt.TextParser)
	return textualParser.TextToMetricFamilies(strings.NewReader(raw))
}
