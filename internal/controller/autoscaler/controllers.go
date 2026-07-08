// Copyright 2025
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package autoscaler

import (
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	knownControllersMutex      sync.Mutex
	knownShardManagers         []schema.GroupVersionKind
	knownMetricValuesProviders []schema.GroupVersionKind
	knownLoadIndexProviders    []schema.GroupVersionKind
	knownPartitionProviders    []schema.GroupVersionKind
	knownReplicaSetControllers []schema.GroupVersionKind
)

const (
	autoscalerAPIGroup                         = "autoscaler.argoproj.io"
	evaluationRefLabel                         = "evaluation_ref"
	evaluationSubsystem                        = "evaluation"
	evaluationTypeLabel                        = "evaluation_type"
	evaluationTypeMostWantedTwoPhaseHysteresis = "most_wanted_two_phase_hysteresis"
	metricsNamespace                           = "argocd_autoscaler"
	metricValuesName                           = "values"
	partitionRefLabel                          = "partition_ref"
	pausedReason                               = "Paused"
	replicaIDLabel                             = "replica_id"
	shardIDLabel                               = "shard_id"
	shardNameLabel                             = "shard_name"
	shardNamespaceLabel                        = "shard_namespace"
	shardServerLabel                           = "shard_server"
	shardUIDLabel                              = "shard_uid"
	v1alpha1Version                            = "v1alpha1"
)

func RegisterShardManager(gvk schema.GroupVersionKind) {
	knownControllersMutex.Lock()
	defer knownControllersMutex.Unlock()
	knownShardManagers = append(knownShardManagers, gvk)
}

func getShardManagers() []schema.GroupVersionKind {
	knownControllersMutex.Lock()
	defer knownControllersMutex.Unlock()
	_copy := make([]schema.GroupVersionKind, len(knownShardManagers))
	copy(_copy, knownShardManagers)
	return _copy
}

func RegisterMetricValuesProvider(gvk schema.GroupVersionKind) {
	knownControllersMutex.Lock()
	defer knownControllersMutex.Unlock()
	knownMetricValuesProviders = append(knownMetricValuesProviders, gvk)
}

func getMetricValuesProviders() []schema.GroupVersionKind {
	knownControllersMutex.Lock()
	defer knownControllersMutex.Unlock()
	_copy := make([]schema.GroupVersionKind, len(knownMetricValuesProviders))
	copy(_copy, knownMetricValuesProviders)
	return _copy
}

func RegisterLoadIndexProvider(gvk schema.GroupVersionKind) {
	knownControllersMutex.Lock()
	defer knownControllersMutex.Unlock()
	knownLoadIndexProviders = append(knownLoadIndexProviders, gvk)
}

func getLoadIndexProviders() []schema.GroupVersionKind {
	knownControllersMutex.Lock()
	defer knownControllersMutex.Unlock()
	_copy := make([]schema.GroupVersionKind, len(knownLoadIndexProviders))
	copy(_copy, knownLoadIndexProviders)
	return _copy
}

func RegisterPartitionProvider(gvk schema.GroupVersionKind) {
	knownControllersMutex.Lock()
	defer knownControllersMutex.Unlock()
	knownPartitionProviders = append(knownPartitionProviders, gvk)
}

func getPartitionProviders() []schema.GroupVersionKind {
	knownControllersMutex.Lock()
	defer knownControllersMutex.Unlock()
	_copy := make([]schema.GroupVersionKind, len(knownPartitionProviders))
	copy(_copy, knownPartitionProviders)
	return _copy
}

func RegisterReplicaSetController(gvk schema.GroupVersionKind) {
	knownControllersMutex.Lock()
	defer knownControllersMutex.Unlock()
	knownReplicaSetControllers = append(knownReplicaSetControllers, gvk)
}

func getReplicaSetControllers() []schema.GroupVersionKind {
	knownControllersMutex.Lock()
	defer knownControllersMutex.Unlock()
	_copy := make([]schema.GroupVersionKind, len(knownReplicaSetControllers))
	copy(_copy, knownReplicaSetControllers)
	return _copy
}

func init() {
	RegisterShardManager(schema.GroupVersionKind{
		Group:   autoscalerAPIGroup,
		Version: v1alpha1Version,
		Kind:    "SecretTypeClusterShardManager",
	})
	RegisterMetricValuesProvider(schema.GroupVersionKind{
		Group:   autoscalerAPIGroup,
		Version: v1alpha1Version,
		Kind:    "PrometheusPoll",
	})
	RegisterMetricValuesProvider(schema.GroupVersionKind{
		Group:   autoscalerAPIGroup,
		Version: v1alpha1Version,
		Kind:    "RobustScalingNormalizer",
	})
	RegisterLoadIndexProvider(schema.GroupVersionKind{
		Group:   autoscalerAPIGroup,
		Version: v1alpha1Version,
		Kind:    "WeightedPNormLoadIndex",
	})
	RegisterPartitionProvider(schema.GroupVersionKind{
		Group:   autoscalerAPIGroup,
		Version: v1alpha1Version,
		Kind:    "LongestProcessingTimePartition",
	})
	RegisterPartitionProvider(schema.GroupVersionKind{
		Group:   autoscalerAPIGroup,
		Version: v1alpha1Version,
		Kind:    "MostWantedTwoPhaseHysteresisEvaluation",
	})
	RegisterReplicaSetController(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "StatefulSet",
	})
}
