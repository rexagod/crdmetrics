package resolver

// Resolver defines behaviors for resolving a given expression.
type Resolver interface {

	// Resolve resolves the given expression.
	// NOTE: The returned map should have a single key:value (query:resolved[LabelValues,Value], of unit length) pair if
	// the expression is resolved to a non-composite value.
	Resolve(string, map[string]interface{}) map[string]string
}
