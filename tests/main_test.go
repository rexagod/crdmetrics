package crdmetrics_test

import (
	"os"
	"testing"
)

const (
	CRDMETRICS_MAIN_PORT = "CRDMETRICS_MAIN_PORT"
	CRDMETRICS_SELF_PORT = "CRDMETRICS_SELF_PORT"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
