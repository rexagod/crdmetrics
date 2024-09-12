# `crdmetrics`: Custom Resource Definition Metrics

[![CI](https://github.com/rexagod/crdmetrics/actions/workflows/continuous-integration.yaml/badge.svg)](https://github.com/rexagod/crdmetrics/actions/workflows/continuous-integration.yaml) [![Go Report Card](https://goreportcard.com/badge/github.com/rexagod/crdmetrics)](https://goreportcard.com/report/github.com/rexagod/crdmetrics) [![Go Reference](https://pkg.go.dev/badge/github.com/rexagod/crdmetrics.svg)](https://pkg.go.dev/github.com/rexagod/crdmetrics)

## Summary

Custom Resource Definition Metrics (`crdmetrics`) is a Kubernetes controller that builds on Kube-State-Metrics' Custom Resource State's ideology and generates metrics for custom resources based on the configuration specified in its managed resource, `CRDMetricsResource`.

The project's [conformance benchmarking](./tests/bench/bench.sh) shows 3x faster RTT for `crdmetrics` as compared to Kube-State-Metrics' Custom Resource Definition Metrics ([f8aa7d9b](https://github.com/kubernetes/kube-state-metrics/commit/f8aa7d9bb9d8e29876e19f4859391a54a7e61d63)) feature-set:

```
Tue Aug 20 21:18:58 IST 2024
[CRDMETRICS]
BUILD:  1021ms
RTT:    1044ms
[KUBESTATEMETRICS]
BUILD:  1042ms
RTT:    3122ms
```

## Development

Start developing by following these steps:

- Set up dependencies with `make setup`.
- Test out your changes with `make apply apply-testdata local`.
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
- [X] Conformance test(s) for Kube-State-Metrics' [Custom Resource State API](https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/extend/customresourcestate-metrics.md#multiple-metricskitchen-sink).
- [X] Benchmark(s) for Kube-State-Metrics' [Custom Resource State API](https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/extend/customresourcestate-metrics.md#multiple-metricskitchen-sink).
- [X] E2E tests covering the controller's basic functionality.
- [X] `s/CRSM/CRDMetrics`.
- [X] [Graduate to ALPHA](https://github.com/kubernetes/enhancements/issues/4785), i.e., draft out a KEP.
- [ ] Make `CRDMetricsResource` namespaced-scope. This allows for:
  - per-namespace configuration (separate configurations between teams), and,
  - garbage collection, since currently the namespace-scoped deployment manages its cluster-scoped resources, which are not garbage collect-able in Kubernetes by design.
- [ ] Meta-metrics for metric generation failures.

###### [License](./LICENSE)
