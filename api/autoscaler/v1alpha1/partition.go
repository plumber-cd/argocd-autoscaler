package v1alpha1

// Partition is a result of sharding by a partitioner.
type Partition struct {
	// Partitioner is the identification of the partitioner that was used for sharding.
	// +kubebuilder:validation:Required
	Partitioner string `json:"partitioner,omitempty"`
	// Replicas is a list of replicas that were created by the partitioner.
	// +kubebuilder:validation:Required
	Replicas []Replica `json:"replicas,omitempty"`
}
