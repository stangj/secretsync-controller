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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SecretsyncSpec defines the desired state of Secretsync.
type SecretsyncSpec struct {
	// 源命名空间
	SourceNamespace string `json:"sourceNamespace"`
	// 源 Secret 名称
	SourceSecretName string `json:"sourceSecretName"`
	// 目标命名空间选择器：支持 Labels 动态选择
	TargetNamespaceSelector *metav1.LabelSelector `json:"targetNamespaceSelector,omitempty"`
	// 目标 Secret 名称（可选，默认与源同名）
	TargetSecretName string `json:"targetSecretName,omitempty"`
	// 显式指定的目标命名空间列表
	TargetNamespaces []string `json:"targetNamespaces,omitempty"`
}

// SecretsyncStatus defines the observed state of Secretsync.
type SecretsyncStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// 已同步命名空间
	SyncedNamespaces []string `json:"syncedNamespaces,omitempty"`
	// 同步失败命名空间
	FailedNamespaces []string `json:"failedNamespaces,omitempty"`
	// 最后同步时间
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Secretsync is the Schema for the secretsyncs API.
type Secretsync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecretsyncSpec   `json:"spec,omitempty"`
	Status SecretsyncStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SecretsyncList contains a list of Secretsync.
type SecretsyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Secretsync `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Secretsync{}, &SecretsyncList{})
}
