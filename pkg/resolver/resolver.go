package resolver

// Resolver defines behaviors for resolving a given expression.
type Resolver interface {

	// Resolve resolves the given expression.
	Resolve(string, map[string]interface{}) string
}
