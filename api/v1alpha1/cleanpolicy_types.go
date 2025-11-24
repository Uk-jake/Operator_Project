/*
Copyright 2025.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CleanPolicySpec defines the desired state of CleanPolicy.
type CleanPolicySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// TTL for finished Jobs (in hours).
	// +kubebuilder:validation:Minimum=1
	TTLHours int32 `json:"ttlHours"`

	// Schedule for cleanup in duration format (e.g. "1h", "30m").
	// This will be parsed with time.ParseDuration.
	// +kubebuilder:validation:Pattern=`^[0-9]+(s|m|h)$`
	Schedule string `json:"schedule"`

	// TargetNamespaces is an optional list of namespaces to scan.
	// If empty or omitted, the operator will only scan the namespace
	// where this CleanPolicy resource exists.
	// +optional
	TargetNamespaces []string `json:"targetNamespaces,omitempty"`

	// JobSelector filters which Jobs are candidates for cleanup.
	// Only Jobs matching this selector will be considered.
	// If nil or empty, all Jobs in the target namespaces are candidates.
	// +optional
	JobSelector *metav1.LabelSelector `json:"jobSelector,omitempty"`
}

// CleanPolicyStatus defines the observed state of CleanPolicy.
type CleanPolicyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// LastCleanupTime is the last time the cleanup ran.
	// +optional
	LastCleanupTime *metav1.Time `json:"lastCleanupTime,omitempty"`

	// Total number of Jobs cleaned by this policy.
	// +optional
	TotalCleanedJobs int64 `json:"totalCleanedJobs,omitempty"`

	// Number of Jobs deleted in the last run.
	// +optional
	LastRunDeleted int32 `json:"lastRunDeleted,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Namespaced
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="TTL(H)",type="integer",JSONPath=".spec.ttlHours"
//+kubebuilder:printcolumn:name="SCHEDULE",type="string",JSONPath=".spec.schedule"
//+kubebuilder:printcolumn:name="LAST_RUN",type="date",JSONPath=".status.lastCleanupTime"

// CleanPolicy is the Schema for the cleanpolicies API.
type CleanPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CleanPolicySpec   `json:"spec,omitempty"`
	Status CleanPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CleanPolicyList contains a list of CleanPolicy.
type CleanPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CleanPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CleanPolicy{}, &CleanPolicyList{})
}
