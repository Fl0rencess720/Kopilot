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

// KopilotSpec defines the desired state of Kopilot
type KopilotSpec struct {
	// +kubebuilder:validation:Required
	Schedule string `json:"schedule"`

	// +kubebuilder:validation:Required
	Selector *metav1.LabelSelector `json:"selector"`

	// +kubebuilder:validation:Required
	LogSource LogSourceSpec `json:"logSource"`

	// +kubebuilder:validation:Required
	LLM LLMSpec `json:"llm"`

	// +kubebuilder:validation:Required
	Notification NotificationSpec `json:"notification"`

	// +optional
	KnowledgeBase *KnowledgeBaseSpec `json:"knowledgeBase,omitempty"`
}

// LogSourceSpec defines the source of logs.
type LogSourceSpec struct {
	// Type specifies the log source type.
	// +kubebuilder:validation:Enum=Kubernetes;Loki
	// +kubebuilder:default:="Kubernetes"
	Type string `json:"type"`

	// This is only used if Type is "Loki".
	// +optional
	Loki *LokiSource `json:"loki,omitempty"`
}

// LokiSource defines connection details for a Loki instance.
type LokiSource struct {
	// +kubebuilder:validation:Required
	Address string `json:"address"`

	// LogQLQuery is the query to execute against Loki to fetch logs.
	// The query can use variables like {pod}, {namespace}, {container}.
	// +optional
	LogQLQuery string `json:"logqlQuery,omitempty"`
}

// LLMSpec defines the AI configuration.
type LLMSpec struct {

	// Model specifies the AI model to be used for analysis.
	// +kubebuilder:default:="gemini"
	Model string `json:"model"`

	// +optional
	Gemini GeminiSpec `json:"gemini"`
}

// GeminiSpec defines Gemini-specific configuration.
type GeminiSpec struct {
	// ModelName is the specific Gemini model to use.
	// +kubebuilder:default:="gemini-2.5-flash"
	ModelName string `json:"modelName"`
	// Thinking enables the AI's reasoning capabilities.
	// +kubebuilder:default:=true
	Thinking bool `json:"thinking"`

	// APIKeySecretRef is a reference to a Kubernetes Secret.
	// The secret must contain a key (e.g., 'apiKey') with the Gemini API key.
	// +kubebuilder:validation:Required
	APIKeySecretRef SecretKeyRef `json:"apiKeySecretRef"`
}

// NotificationSpec defines where and how to send notifications.
type NotificationSpec struct {
	// Sinks is a list of notification channels.
	// +kubebuilder:validation:MinItems=1
	Sinks []NotificationSink `json:"sinks"`
}

// NotificationSink defines a single notification channel.
type NotificationSink struct {
	// Name is a unique identifier for this sink.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Feishu configures notifications to a Feishu (Lark) webhook.
	// In the future, you could add other types like Slack here.
	// +optional
	Feishu *FeishuSink `json:"feishu,omitempty"`
}

// FeishuSink defines the configuration for a Feishu webhook.
type FeishuSink struct {
	// WebhookSecretRef is a reference to a Kubernetes Secret.
	// The secret must contain a key (e.g., 'url') with the Feishu webhook URL.
	// +kubebuilder:validation:Required
	WebhookSecretRef SecretKeyRef `json:"webhookSecretRef"`

	// SignatureSecretRef is a reference to a Kubernetes Secret that holds the webhook signature.
	// The secret should contain a key (e.g., 'signature') with the Feishu webhook signature.
	// This is required for webhooks that use custom signatures for security.
	// +kubebuilder:validation:Required
	SignatureSecretRef SecretKeyRef `json:"signatureSecretRef"`
}

// SecretKeyRef is a reference to a key within a Kubernetes Secret.
type SecretKeyRef struct {
	// Namespace is the namespace where the Secret is located.
	// If not specified, defaults to the same namespace as the Kopilot instance.
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// Name of the Secret.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Key within the Secret.
	// +kubebuilder:validation:Required
	Key string `json:"key"`
}

// KnowledgeBaseSpec is a placeholder for Day 6.
type KnowledgeBaseSpec struct {
	// ConfigMapRef references a ConfigMap containing knowledge base articles.
	// +optional
	ConfigMapRef *string `json:"configMapRef,omitempty"`
}

// KopilotStatus defines the observed state of Kopilot.
type KopilotStatus struct {
	// Conditions store the status of the Kopilot instance.
	// This is the modern way to represent status in Kubernetes.
	// +optional
	// +patchStrategy=merge
	// +patchMergeKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// LastCheckTime is the timestamp of the last log fetch.
	// +optional
	LastCheckTime *metav1.Time `json:"lastCheckTime,omitempty"`

	// LastAnalysisResult is a summary of the latest AI analysis.
	// +optional
	LastAnalysisResult string `json:"lastAnalysisResult,omitempty"`

	// LastError records the last error encountered by the operator for this instance.
	// +optional
	LastError string `json:"lastError,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Schedule",type="string",JSONPath=".spec.schedule"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].reason"
// +kubebuilder:printcolumn:name="Last Check",type="date",JSONPath=".status.lastCheckTime"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Kopilot is the Schema for the kopilots API
type Kopilot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KopilotSpec   `json:"spec,omitempty"`
	Status KopilotStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KopilotList contains a list of Kopilot
type KopilotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Kopilot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Kopilot{}, &KopilotList{})
}
