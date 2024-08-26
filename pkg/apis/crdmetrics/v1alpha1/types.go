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
// +kubebuilder:resource:singular=crdmetricsresource,scope=Cluster,shortName=crdmr
// +kubebuilder:rbac:groups=crdmetrics.instrumentation.k8s-sigs.io,resources=crdmetricsresources;crdmetricsresources/status,verbs=*
// +kubebuilder:subresource:status

// CRDMetricsResource is a specification for a CRDMetricsResource resource.
type CRDMetricsResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              CRDMetricsResourceSpec   `json:"spec"`
	Status            CRDMetricsResourceStatus `json:"status,omitempty"`
}

// CRDMetricsResourceSpec is the spec for a CRDMetricsResource resource.
type CRDMetricsResourceSpec struct {

	// +kubebuilder:validation:Format=string
	// +kubebuilder:validation:Required
	// +required

	// ConfigurationYAML is the crdmetrics configuration that generates metrics.
	ConfigurationYAML string `json:"configurationYAML"`
}

// +kubebuilder:validation:Optional
// +optional

// CRDMetricsResourceStatus is the status for a CRDMetricsResource resource.
type CRDMetricsResourceStatus struct {

	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type

	// Conditions is an array of conditions associated with the resource.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Set sets the given condition for the resource.
func (status *CRDMetricsResourceStatus) Set(
	resource *CRDMetricsResource,
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

// CRDMetricsResourceList is a list of CRDMetricsResource resources.
type CRDMetricsResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CRDMetricsResource `json:"items"`
}
