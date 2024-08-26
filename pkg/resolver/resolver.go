/*
Copyright 2024 The Kubernetes crdmetrics Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resolver

// Resolver defines behaviors for resolving a given expression.
type Resolver interface {

	// Resolve resolves the given expression.
	// NOTE: The returned map should have a single key:value (query:resolved[LabelValues,Value], of unit length) pair if
	// the expression is resolved to a non-composite value.
	Resolve(string, map[string]interface{}) map[string]string
}
