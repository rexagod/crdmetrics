package tests

import (
	"os"
	"testing"
)

const (
	CRSM_MAIN_PORT = "CRSM_MAIN_PORT"
	CRSM_SELF_PORT = "CRSM_SELF_PORT"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
