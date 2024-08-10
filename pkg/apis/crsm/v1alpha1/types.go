/*
Copyright 2024 The Kubernetes CRSM Authors.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:rbac:groups=crsm.instrumentation.k8s-sigs.io,resources=customresourcestatemetricsresources;customresourcestatemetricsresources/status,verbs=*
// +kubebuilder:resource:shortName=crsmr
// +kubebuilder:subresource:status

// CustomResourceStateMetricsResource is a specification for a CustomResourceStateMetricsResource resource.
type CustomResourceStateMetricsResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              CustomResourceStateMetricsResourceSpec   `json:"spec"`
	Status            CustomResourceStateMetricsResourceStatus `json:"status,omitempty"`
}

// CustomResourceStateMetricsResourceSpec is the spec for a CustomResourceStateMetricsResource resource.
type CustomResourceStateMetricsResourceSpec struct {

	// +kubebuilder:validation:Optional
	// +optional

	// Configuration is the CEL configuration that generates metrics.
	Configuration string `json:"customResourceStateMetricsConfiguration"`
}

// +kubebuilder:validation:Optional
// +optional

// CustomResourceStateMetricsResourceStatus is the status for a CustomResourceStateMetricsResource resource.
type CustomResourceStateMetricsResourceStatus struct {

	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:Optional
	// +optional

	// Conditions is an array of conditions associated with the resource.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// CustomResourceStateMetricsResourceList is a list of CustomResourceStateMetricsResource resources.
type CustomResourceStateMetricsResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CustomResourceStateMetricsResource `json:"items"`
}
