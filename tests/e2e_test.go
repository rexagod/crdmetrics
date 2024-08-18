package crsm_test

import (
	"net/url"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/rexagod/crsm/tests/framework"
)

func TestMainServer(t *testing.T) {
	t.Parallel()

	// Test if /metrics response is as expected.
	r := framework.NewRunner()
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
kube_customresource_platform_info{language="csharp",environmenttype="dev",group="contoso.com",version="v1alpha1",kind="MyPlatform"} 1.000000
# HELP kube_customresource_platform_replicas Number of replicas for each MyPlatform instance
# TYPE kube_customresource_platform_replicas gauge
kube_customresource_platform_replicas{name="test-dotnet-app",dynamicnoresolveshouldremainthesame_compositeunsupportedupstream="map[bar:2 foo:1 job:crsm]",group="contoso.com",version="v1alpha1",kind="MyPlatform"} 3.000000
# HELP kube_customresource_foos_info Information about each Foo instance
# TYPE kube_customresource_foos_info gauge
kube_customresource_foos_info{static="42",dynamicshouldresolvetoname="example-foo",dynamicnoresolveshouldremainthesame1="o.metadata.labels.baz",dynamicnoresolveshouldremainthesame2="metadata.labels.baz",group="samplecontroller.k8s.io",version="v1alpha1",kind="Foo"} 42.000000
# HELP kube_customresource_foo_replicas Number of replicas for each Foo instance
# TYPE kube_customresource_foo_replicas gauge
kube_customresource_foo_replicas{name="example-foo",group="samplecontroller.k8s.io",version="v1alpha1",kind="Foo"} 1.000000
# HELP kube_customresource_platform_info_conformance Information about each MyPlatform instance (using existing exhaustive CRS feature-set for conformance)
# TYPE kube_customresource_platform_info_conformance gauge
kube_customresource_platform_info_conformance{id="1000",os="linux",job="crsm",name="test-dotnet-app",appid="testdotnetapp",language="csharp",label_bar="2",label_foo="1",label_job="crsm",instancesize="small",environmenttype="dev",group="contoso.com",version="v1alpha1",kind="MyPlatform"} 2.000000
`
	if equal := cmp.Equal(gotRaw, wantRaw); !equal {
		t.Fatalf("[-got +want]:\n%s", cmp.Diff(gotRaw, wantRaw))
	}
}

func TestSelfServer(t *testing.T) {
	t.Parallel()

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
	inFlightDurationFamily, ok := telemetryMetrics[httpRequestDurationSeconds]
	if ok {
		inFlightDurationTotal = inFlightDurationFamily.GetMetric()[0].GetHistogram().GetSampleSum()
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
	newInFlightDurationTotal := telemetryMetrics[httpRequestDurationSeconds].GetMetric()[0].GetHistogram().GetSampleSum()
	if newInFlightDurationTotal == inFlightDurationTotal {
		t.Fatalf("got in-flight duration total %f, want %f", newInFlightDurationTotal, inFlightDurationTotal)
	}
}
