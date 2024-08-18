package crsm_test

import (
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/rexagod/crsm/tests/framework"
)

// TestConformance is a subset of TestMainServer that only checks for conformance metrics.
func TestConformance(t *testing.T) {
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
	shouldContainRaw := `# HELP kube_customresource_platform_info_conformance Information about each MyPlatform instance (using existing exhaustive CRS feature-set for conformance)
# TYPE kube_customresource_platform_info_conformance gauge
kube_customresource_platform_info_conformance{id="1000",os="linux",job="crsm",name="test-dotnet-app",appid="testdotnetapp",language="csharp",label_bar="2",label_foo="1",label_job="crsm",instancesize="small",environmenttype="dev",group="contoso.com",version="v1alpha1",kind="MyPlatform"} 2.000000
`
	if !strings.Contains(gotRaw, shouldContainRaw) {
		t.Fatalf("response does not contain expected conformance metrics:\n\tgot:\n%s\n\tshould contain:\n%s\n", gotRaw, shouldContainRaw)
	}
}
