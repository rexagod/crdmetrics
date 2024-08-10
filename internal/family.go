package internal

import (
	"fmt"
	"strings"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// family represents a metric family (a group of metrics with the same name).
type family struct {

	// name is the name of the metric family.
	name string

	// t is the type of the metric family.
	// NOTE: This will always be pinned to `gauge`.
	t string

	// metrics is a slice of metrics that belong to the metric family.
	metrics []*metric
}

// raw returns the given family in its byte representation.
func (f family) raw() []byte {
	s := strings.Builder{}
	for _, m := range f.metrics {
		s.WriteString(f.name)
		err := m.writeTo(&s)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("error writing %s metric: %w", f.name, err))
		}
	}

	return []byte(s.String())
}

// metricFamilyGeneratorType is the metric family generator type.
type metricFamilyGeneratorType func(interface{}) *family

// familyGenerator generates a metric family from an object.
type familyGenerator struct {
	name                          string
	help                          string
	t                             string
	objectToMetricFamilyGenerator metricFamilyGeneratorType
}

// newFamilyGenerator returns a new familyGenerator.
func newFamilyGenerator(name, help, t string, objectToMetricFamilyGenerator metricFamilyGeneratorType) *familyGenerator {
	return &familyGenerator{
		name:                          name,
		help:                          help,
		t:                             t,
		objectToMetricFamilyGenerator: objectToMetricFamilyGenerator,
	}
}

// generateFamily generates a *family from the given object.
func (fg *familyGenerator) generateFamily(object interface{}) *family {
	generatedFamily := fg.objectToMetricFamilyGenerator(object)
	generatedFamily.name = fg.name
	generatedFamily.t = fg.t
	return generatedFamily
}

// generateHeaderString generates the header for the given family generator.
func (fg *familyGenerator) generateHeaderString() string {
	header := strings.Builder{}
	header.WriteString("# HELP ")
	header.WriteString(fg.name)
	header.WriteByte(' ')
	header.WriteString(fg.help)
	header.WriteByte('\n')
	header.WriteString("# TYPE ")
	header.WriteString(fg.name)
	header.WriteByte(' ')
	header.WriteString(fg.t)

	return header.String()
}
