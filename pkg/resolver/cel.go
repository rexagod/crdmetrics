package resolver

import (
	"fmt"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/interpreter"
)

// CELResolver represents a resolver for CEL expressions.
type CELResolver struct {
	logger klog.Logger
}

// CELResolver implements the Resolver interface.
var _ Resolver = &CELResolver{}

// NewCELResolver returns a new CEL resolver.
func NewCELResolver() *CELResolver {
	return &CELResolver{}
}

// costEstimator helps estimate the runtime cost of CEL queries.
type costEstimator struct{}

// costEstimator implements the ActualCostEstimator interface.
var _ interpreter.ActualCostEstimator = costEstimator{}

// CallCost helps set the runtime cost for CEL queries on a per-function basis. This affects `ActualCost()` directly.
// function: The function name.
// args: The arguments passed to the function.
// result: The return value of the function.
func (ce costEstimator) CallCost(function, _ string, args []ref.Val, result ref.Val) *uint64 {
	estimatedCost := uint64(1)
	customFunctionsCosts := map[string]uint64{}
	estimatedCost += customFunctionsCosts[function]

	return &estimatedCost
}

// resolverPrinter knows how to handle various types returned by the CEL evaluator. This is used instead of CEL's
// `ConvertToNative` to be able to define custom printing logic for types that the API doesn't support (for e.g., maps).
type resolverPrinter struct {
	Name string
	Out  ref.Val
}

// resolverPrinter implements fmt.Formatter.
var _ fmt.Formatter = resolverPrinter{}

// Format formats the given verb.
// nolint: godox
// TODO: If the output is not a non-composite, create label keys and values (using the given labelKey as prefix, which
// also implies labelKeys maybe empty)?
func (rp resolverPrinter) Format(f fmt.State, verb rune) {
	s := ""
	switch verb {
	case 'v':
		switch rp.Out.Type() {
		case types.ListType:
			l := rp.Out.Value().([]interface{})
			for _, el := range l {
				s += fmt.Sprintf("%v ", el)
			}
		case types.MapType:
			m := rp.Out.Value().(map[string]interface{})
			for _, v := range m {
				switch v.(type) {

				// Only print non-composites' values for maps.
				case string, int, float64, bool:
					s += fmt.Sprintf("%v ", v)
				default:
					utilruntime.HandleError(fmt.Errorf("unsupported type %T in map", v))
				}
			}
		default: // For all other types, print the %v representation as is.
			s = fmt.Sprintf("%v", rp.Out.Value())
		}
	default: // For all other verbs, print the %v representation as is.
		n, err := fmt.Fprintf(f, "%v", rp.Out.Value())
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("error formatting %q after %d bytes: %w", rp.Name, n, err))
		}
		return
	}

	n, err := fmt.Fprintf(f, "%s", s)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error formatting %q after %d bytes: %w", rp.Name, n, err))
	}
}

// Resolve resolves the given query against the given unstructured object.
func (cr *CELResolver) Resolve(query string, unstructuredObjectMap map[string]interface{}) string {
	cr.logger = cr.logger.WithValues("query", query)

	// Create a custom CEL environment.
	// nolint: godox
	// TODO: Investigate if this is a potential bottleneck.
	env, err := cel.NewEnv(
		cel.CrossTypeNumericComparisons(true),
		cel.DefaultUTCTimeZone(true),
		cel.EagerlyValidateDeclarations(true),
	)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error creating CEL environment: %w", err))
		return query
	}

	// Parse.
	ast, iss := env.Parse(query)
	if iss.Err() != nil {
		utilruntime.HandleError(fmt.Errorf("error parsing CEL query: %w", iss.Err()))
		return query
	}

	// Compile.
	// Assume a couple of points on average set by CEL standard library for their exposed APIs.
	// Points for function overloads defined by us are set in the `CallCost` method.
	const costLimit = 25
	var program cel.Program
	program, err = env.Program(
		ast,
		cel.CostLimit(costLimit),
		cel.CostTracking(new(costEstimator)),
	)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error compiling CEL query: %w", err))
		return query
	}

	// Inject the object and evaluate.
	var out ref.Val
	var evalDetails *cel.EvalDetails
	out, evalDetails, err = program.Eval(map[string]interface{}{
		"o" /* Queries will follow the format: o.<A>.<AB>.<ABC>... */ : unstructuredObjectMap,
	})
	cr.logger = cr.logger.WithValues(
		"costLimit", costLimit,
	)
	if evalDetails != nil {
		cr.logger = cr.logger.WithValues(
			"queryCost", *evalDetails.ActualCost(),
		)
	}
	if err != nil {
		cr.logger.Info("ignoring resolution for query")
		return query
	}
	cr.logger.V(4).Info("CEL query runtime cost")

	return fmt.Sprintf("%v", resolverPrinter{
		Name: query,
		Out:  out,
	})
}
