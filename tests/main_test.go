package crdmetrics_test

import (
	"os"
	"testing"
)

const (
	CRDMetricsMainPort = "CRDMETRICS_MAIN_PORT"
	CRDMetricsSelfPort = "CRDMETRICS_SELF_PORT"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
