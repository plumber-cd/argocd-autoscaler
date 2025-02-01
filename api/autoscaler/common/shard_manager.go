package common

import "sigs.k8s.io/controller-runtime/pkg/client"

type ShardManagerSpec struct {
	// Replicas is the list of replicas with shard assignments.
	Replicas ReplicaList `json:"replicas,omitempty"`
}

type ShardManagerStatus struct {
	ShardsProviderStatus `json:",inline"`
	// Replicas is the list of replicas that was last successfully applied by the manager.
	Replicas ReplicaList `json:"replicas,omitempty"`
}

// +k8s:deepcopy-gen=false
type ShardManager interface {
	GetSpec() *ShardManagerSpec
	GetStatus() *ShardManagerStatus
	GetClientObject() client.Object
}
