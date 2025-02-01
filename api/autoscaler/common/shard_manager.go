package common

type ShardManagerSpec struct {
	// Replicas is the list of replicas with shard assignments.
	Replicas []Replica `json:"replicas,omitempty"`
}

// +k8s:deepcopy-gen=false
type ShardManager interface {
	GetSpec() ShardManagerSpec
}
