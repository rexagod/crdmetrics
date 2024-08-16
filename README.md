# `crsm`: Custom Resource State Metrics

[![CI](https://github.com/rexagod/crsm/actions/workflows/continuous-integration.yaml/badge.svg)](https://github.com/rexagod/crsm/actions/workflows/continuous-integration.yaml) [![Go Report Card](https://goreportcard.com/badge/github.com/rexagod/crsm)](https://goreportcard.com/report/github.com/rexagod/crsm) [![Go Reference](https://pkg.go.dev/badge/github.com/rexagod/crsm.svg)](https://pkg.go.dev/github.com/rexagod/crsm)

## Summary

Custom Resource State Metrics (`crsm`) is a Kubernetes controller that builds on Kube-State-Metrics' Custom Resource State's ideology and generates metrics for custom resources based on the configuration specified in its managed resource, `CustomResourceStateMetricsResource`.

## Development

Start developing by following these steps:

- Set up dependencies with `make setup`.
- Test out your changes with `POD_NAMESPACE=<controller-namespace> make apply apply-testdata local`.
  - Telemetry metrics, by default, are exposed on `:9998/metrics`.
  - Resource metrics, by default, are exposed on `:9999/metrics`.
- Start a `pprof` interactive session with `make pprof`.

For more details, take a look at the [Makefile](Makefile) targets.

## Notes

- Garbage in, garbage out: Invalid configurations will generate invalid metrics. The exception to this being that certain checks that ensure metric structure are still present (for e.g., `value` should be a `float64`).
- Library support: The module is **never** intended to be used as a library, and as such, does not export any functions or types, with `pkg/` being an exception (for managed types and such).
- Metrics stability: There are no metrics [stability](https://kubernetes.io/blog/2021/04/23/kubernetes-release-1.21-metrics-stability-ga/) guarantees, as the metrics are user-generated.
- No middle-ware: The configuration is `unmarshal`led into a set of stores that the codebase directly operates on. There is no middle-ware that processes the configuration before it is used, in order to avoid unnecessary complexity. However, the expression(s) within the `value` and `labelValues` may need to be evaluated before being used, and as such, are exceptions.

## TODO

In the order of priority:

- [X] CEL expressions for metric generation (or [*unstructured.Unstructured](https://github.com/kubernetes/apimachinery/issues/181), if that suffices).
- [ ] Conformance tests and benchmarks for Kube-State-Metrics' [Custom Resource State API](https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/extend/customresourcestate-metrics.md#multiple-metricskitchen-sink).
- [X] E2E tests covering the controller's basic functionality.
- [ ] [Graduate to ALPHA](https://github.com/kubernetes/enhancements/issues/4785).
- [ ] gRPC server for metrics generation.

###### [License](./LICENSE)
