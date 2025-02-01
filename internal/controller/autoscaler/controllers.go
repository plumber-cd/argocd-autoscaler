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

func init() {
	RegisterShardManager(schema.GroupVersionKind{
		Group:   "autoscaler.argoproj.io",
		Version: "v1alpha1",
		Kind:    "SecretTypeClusterShardManager",
	})
	RegisterMetricValuesProvider(schema.GroupVersionKind{
		Group:   "autoscaler.argoproj.io",
		Version: "v1alpha1",
		Kind:    "PrometheusPoll",
	})
	RegisterMetricValuesProvider(schema.GroupVersionKind{
		Group:   "autoscaler.argoproj.io",
		Version: "v1alpha1",
		Kind:    "RobustScalingNormalizer",
	})
	RegisterLoadIndexProvider(schema.GroupVersionKind{
		Group:   "autoscaler.argoproj.io",
		Version: "v1alpha1",
		Kind:    "WeightedPNormLoadIndex",
	})
	RegisterPartitionProvider(schema.GroupVersionKind{
		Group:   "autoscaler.argoproj.io",
		Version: "v1alpha1",
		Kind:    "LongestProcessingTimePartition",
	})
}
