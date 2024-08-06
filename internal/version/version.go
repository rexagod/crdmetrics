// Package version prints the version metadata of the binary.
package version

import (
	"github.com/prometheus/common/version"
)

func Version() string {
	return version.Print("metrics-anomaly-detector")
}
