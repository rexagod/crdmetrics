// Package version prints the version metadata of the binary.
package version

import (
	"github.com/prometheus/common/version"
)

// ControllerName is used in metrics as is, so snake-case is necessary.
const ControllerName = "custom_resource_state_metrics"

func Version() string {
	return version.Print(ControllerName)
}
