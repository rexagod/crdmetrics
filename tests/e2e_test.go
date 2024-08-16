package tests

import (
	"net/url"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/rexagod/crsm/tests/framework"
)

func TestMainServer(t *testing.T) {
	r := framework.NewRunner()

	// Test if /metrics response is as expected.
	mainPort, found := os.LookupEnv(CRSM_MAIN_PORT)
	if !found {
		t.Fatal(CRSM_MAIN_PORT + "is not set")
	}
	mainMetricsURL := &url.URL{
		Host:   "localhost:" + mainPort,
		Path:   "/metrics",
		Scheme: "http",
	}
	gotRaw, err := r.GetRaw(mainMetricsURL)
	if err != nil {
		t.Fatalf("failed to parse metrics: %v", err)
	}
	wantRaw := `# HELP kube_customresource_platform_info Information about each MyPlatform instance
# TYPE kube_customresource_platform_info gauge
kube_customresource_platform_info{name="test-dotnet-app",group="contoso.com",version="v1alpha1",kind="MyPlatform"} 2.000000
kube_customresource_platform_info{environmentType="dev",language="csharp",group="contoso.com",version="v1alpha1",kind="MyPlatform"} 1.000000
# HELP kube_customresource_platform_replicas Number of replicas for each MyPlatform instance
# TYPE kube_customresource_platform_replicas gauge
kube_customresource_platform_replicas{name="test-dotnet-app",group="contoso.com",version="v1alpha1",kind="MyPlatform"} 3.000000
# HELP kube_customresource_foos_info Information about each Foo instance
# TYPE kube_customresource_foos_info gauge
kube_customresource_foos_info{dynamicShouldResolveToName="example-foo",static="42",dynamicNoResolveShouldRemainTheSame1="o.metadata.labels.baz",dynamicNoResolveShouldRemainTheSame2="metadata.labels.baz",group="samplecontroller.k8s.io",version="v1alpha1",kind="Foo"} 42.000000
# HELP kube_customresource_foo_replicas Number of replicas for each Foo instance
# TYPE kube_customresource_foo_replicas gauge
kube_customresource_foo_replicas{name="example-foo",group="samplecontroller.k8s.io",version="v1alpha1",kind="Foo"} 1.000000
`
	if equal := cmp.Equal(gotRaw, wantRaw); !equal {
		t.Fatalf("[-got +want]:\n%s", cmp.Diff(gotRaw, wantRaw))
	}
}

func TestSelfServer(t *testing.T) {
	r := framework.NewRunner()
	const httpRequestDurationSeconds = "http_request_duration_seconds"

	// Fetch the recorded in-flight time for main /metrics endpoint.
	selfPort, found := os.LookupEnv(CRSM_SELF_PORT)
	if !found {
		t.Fatal(CRSM_SELF_PORT + "is not set")
	}
	selfMetricsURL := &url.URL{
		Host:   "localhost:" + selfPort,
		Path:   "/metrics",
		Scheme: "http",
	}
	telemetryMetrics, err := r.GetMetrics(selfMetricsURL)
	if err != nil {
		t.Fatalf("failed to get metrics: %v", err)
	}
	inFlightDurationTotal := 0.0
	inFlightDurationTotalPtr := telemetryMetrics[httpRequestDurationSeconds].Metric[0].Histogram.SampleSum
	if inFlightDurationTotalPtr != nil {
		inFlightDurationTotal = *inFlightDurationTotalPtr
	}

	// Ping main /metrics endpoint.
	mainPort, found := os.LookupEnv("CRSM_MAIN_PORT")
	if !found {
		t.Fatal("CRSM_MAIN_PORT is not set")
	}
	mainURL := &url.URL{
		Host:   "localhost:" + mainPort,
		Path:   "/metrics",
		Scheme: "http",
	}
	_, err = r.GetRaw(mainURL)
	if err != nil {
		t.Fatalf("failed to get metrics: %v", err)
	}

	// Check if the recorded in-flight time for main /metrics requests increased.
	telemetryMetrics, err = r.GetMetrics(selfMetricsURL)
	if err != nil {
		t.Fatalf("failed to get metrics: %v", err)
	}
	newInFlightDurationTotal := *telemetryMetrics[httpRequestDurationSeconds].Metric[0].Histogram.SampleSum
	if newInFlightDurationTotal == inFlightDurationTotal {
		t.Fatalf("got in-flight duration total %f, want %f", newInFlightDurationTotal, inFlightDurationTotal)
	}
}
