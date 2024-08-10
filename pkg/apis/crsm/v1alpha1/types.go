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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/strings/slices"
)

const (

	// ConditionTypeProcessed represents the condition type for a resource that has been processed successfully.
	ConditionTypeProcessed = iota

	// ConditionTypeFailed represents the condition type for resource that has failed to process further.
	ConditionTypeFailed
)

var (

	// ConditionType is a slice of strings representing the condition types.
	ConditionType = []string{"Processed", "Failed"}

	// ConditionMessageTrue is a group of condition messages applicable when the associated condition status is true.
	ConditionMessageTrue = []string{
		"Resource configuration has been processed successfully",
		"Resource failed to process",
	}

	// ConditionMessageFalse is a group of condition messages applicable when the associated condition status is false.
	ConditionMessageFalse = []string{
		"Resource configuration is yet to be processed",
		"N/A",
	}

	// ConditionReasonTrue is a group of condition reasons applicable when the associated condition status is true.
	ConditionReasonTrue = []string{"EventHandlerSucceeded", "EventHandlerFailed"}

	// ConditionReasonFalse is a group of condition reasons applicable when the associated condition status is false.
	ConditionReasonFalse = []string{"EventHandlerRunning", "N/A"}
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

	// +kubebuilder:validation:Format=string
	// +kubebuilder:validation:Required
	// +required

	// ConfigurationYAML is the CRSMR configuration that generates metrics.
	ConfigurationYAML string `json:"customResourceStateMetricsConfigurationYAML"`
}

// +kubebuilder:validation:Optional
// +optional

// CustomResourceStateMetricsResourceStatus is the status for a CustomResourceStateMetricsResource resource.
type CustomResourceStateMetricsResourceStatus struct {

	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type

	// Conditions is an array of conditions associated with the resource.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Set sets the given condition for the resource.
func (status *CustomResourceStateMetricsResourceStatus) Set(
	resource *CustomResourceStateMetricsResource,
	condition metav1.Condition,
) {

	// Prefix condition messages with consistent hints.
	var message, reason string
	conditionTypeNumeric := slices.Index(ConditionType, condition.Type)
	if condition.Status == metav1.ConditionTrue {
		reason = ConditionReasonTrue[conditionTypeNumeric]
		message = ConditionMessageTrue[conditionTypeNumeric]
	} else {
		reason = ConditionReasonFalse[conditionTypeNumeric]
		message = ConditionMessageFalse[conditionTypeNumeric]
	}

	// Populate status fields.
	condition.Reason = reason
	condition.Message = fmt.Sprintf("%s: %s", message, condition.Message)
	condition.LastTransitionTime = metav1.Now()
	condition.ObservedGeneration = resource.GetGeneration()

	// Check if the condition already exists.
	for i, existingCondition := range status.Conditions {
		if existingCondition.Type == condition.Type {
			// Update the existing condition.
			status.Conditions[i] = condition
			return
		}
	}

	// Append the new condition if it does not exist (+listMapKey=type).
	status.Conditions = append(status.Conditions, condition)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// CustomResourceStateMetricsResourceList is a list of CustomResourceStateMetricsResource resources.
type CustomResourceStateMetricsResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CustomResourceStateMetricsResource `json:"items"`
}
