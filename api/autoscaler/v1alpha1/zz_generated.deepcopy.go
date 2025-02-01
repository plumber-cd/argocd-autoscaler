//go:build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LongestProcessingTimePartition) DeepCopyInto(out *LongestProcessingTimePartition) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LongestProcessingTimePartition.
func (in *LongestProcessingTimePartition) DeepCopy() *LongestProcessingTimePartition {
	if in == nil {
		return nil
	}
	out := new(LongestProcessingTimePartition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LongestProcessingTimePartition) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LongestProcessingTimePartitionList) DeepCopyInto(out *LongestProcessingTimePartitionList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]LongestProcessingTimePartition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LongestProcessingTimePartitionList.
func (in *LongestProcessingTimePartitionList) DeepCopy() *LongestProcessingTimePartitionList {
	if in == nil {
		return nil
	}
	out := new(LongestProcessingTimePartitionList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LongestProcessingTimePartitionList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LongestProcessingTimePartitionSpec) DeepCopyInto(out *LongestProcessingTimePartitionSpec) {
	*out = *in
	in.PartitionerSpec.DeepCopyInto(&out.PartitionerSpec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LongestProcessingTimePartitionSpec.
func (in *LongestProcessingTimePartitionSpec) DeepCopy() *LongestProcessingTimePartitionSpec {
	if in == nil {
		return nil
	}
	out := new(LongestProcessingTimePartitionSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LongestProcessingTimePartitionStatus) DeepCopyInto(out *LongestProcessingTimePartitionStatus) {
	*out = *in
	in.PartitionProviderStatus.DeepCopyInto(&out.PartitionProviderStatus)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LongestProcessingTimePartitionStatus.
func (in *LongestProcessingTimePartitionStatus) DeepCopy() *LongestProcessingTimePartitionStatus {
	if in == nil {
		return nil
	}
	out := new(LongestProcessingTimePartitionStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MostWantedTwoPhaseHysteresisEvaluation) DeepCopyInto(out *MostWantedTwoPhaseHysteresisEvaluation) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MostWantedTwoPhaseHysteresisEvaluation.
func (in *MostWantedTwoPhaseHysteresisEvaluation) DeepCopy() *MostWantedTwoPhaseHysteresisEvaluation {
	if in == nil {
		return nil
	}
	out := new(MostWantedTwoPhaseHysteresisEvaluation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MostWantedTwoPhaseHysteresisEvaluation) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MostWantedTwoPhaseHysteresisEvaluationList) DeepCopyInto(out *MostWantedTwoPhaseHysteresisEvaluationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]MostWantedTwoPhaseHysteresisEvaluation, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MostWantedTwoPhaseHysteresisEvaluationList.
func (in *MostWantedTwoPhaseHysteresisEvaluationList) DeepCopy() *MostWantedTwoPhaseHysteresisEvaluationList {
	if in == nil {
		return nil
	}
	out := new(MostWantedTwoPhaseHysteresisEvaluationList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MostWantedTwoPhaseHysteresisEvaluationList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MostWantedTwoPhaseHysteresisEvaluationSpec) DeepCopyInto(out *MostWantedTwoPhaseHysteresisEvaluationSpec) {
	*out = *in
	in.EvaluatorSpec.DeepCopyInto(&out.EvaluatorSpec)
	out.PollingPeriod = in.PollingPeriod
	out.StabilizationPeriod = in.StabilizationPeriod
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MostWantedTwoPhaseHysteresisEvaluationSpec.
func (in *MostWantedTwoPhaseHysteresisEvaluationSpec) DeepCopy() *MostWantedTwoPhaseHysteresisEvaluationSpec {
	if in == nil {
		return nil
	}
	out := new(MostWantedTwoPhaseHysteresisEvaluationSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MostWantedTwoPhaseHysteresisEvaluationStatus) DeepCopyInto(out *MostWantedTwoPhaseHysteresisEvaluationStatus) {
	*out = *in
	in.PartitionProviderStatus.DeepCopyInto(&out.PartitionProviderStatus)
	if in.History != nil {
		in, out := &in.History, &out.History
		*out = make([]MostWantedTwoPhaseHysteresisEvaluationStatusHistoricalRecord, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.LastEvaluationTimestamp != nil {
		in, out := &in.LastEvaluationTimestamp, &out.LastEvaluationTimestamp
		*out = (*in).DeepCopy()
	}
	if in.Projection != nil {
		in, out := &in.Projection, &out.Projection
		*out = make([]common.Replica, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MostWantedTwoPhaseHysteresisEvaluationStatus.
func (in *MostWantedTwoPhaseHysteresisEvaluationStatus) DeepCopy() *MostWantedTwoPhaseHysteresisEvaluationStatus {
	if in == nil {
		return nil
	}
	out := new(MostWantedTwoPhaseHysteresisEvaluationStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MostWantedTwoPhaseHysteresisEvaluationStatusHistoricalRecord) DeepCopyInto(out *MostWantedTwoPhaseHysteresisEvaluationStatusHistoricalRecord) {
	*out = *in
	in.Timestamp.DeepCopyInto(&out.Timestamp)
	if in.Replicas != nil {
		in, out := &in.Replicas, &out.Replicas
		*out = make([]common.Replica, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MostWantedTwoPhaseHysteresisEvaluationStatusHistoricalRecord.
func (in *MostWantedTwoPhaseHysteresisEvaluationStatusHistoricalRecord) DeepCopy() *MostWantedTwoPhaseHysteresisEvaluationStatusHistoricalRecord {
	if in == nil {
		return nil
	}
	out := new(MostWantedTwoPhaseHysteresisEvaluationStatusHistoricalRecord)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PrometheusMetric) DeepCopyInto(out *PrometheusMetric) {
	*out = *in
	if in.NoData != nil {
		in, out := &in.NoData, &out.NoData
		x := (*in).DeepCopy()
		*out = &x
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PrometheusMetric.
func (in *PrometheusMetric) DeepCopy() *PrometheusMetric {
	if in == nil {
		return nil
	}
	out := new(PrometheusMetric)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PrometheusPoll) DeepCopyInto(out *PrometheusPoll) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PrometheusPoll.
func (in *PrometheusPoll) DeepCopy() *PrometheusPoll {
	if in == nil {
		return nil
	}
	out := new(PrometheusPoll)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PrometheusPoll) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PrometheusPollList) DeepCopyInto(out *PrometheusPollList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PrometheusPoll, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PrometheusPollList.
func (in *PrometheusPollList) DeepCopy() *PrometheusPollList {
	if in == nil {
		return nil
	}
	out := new(PrometheusPollList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PrometheusPollList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PrometheusPollSpec) DeepCopyInto(out *PrometheusPollSpec) {
	*out = *in
	in.PollerSpec.DeepCopyInto(&out.PollerSpec)
	out.InitialDelay = in.InitialDelay
	out.Period = in.Period
	if in.Metrics != nil {
		in, out := &in.Metrics, &out.Metrics
		*out = make([]PrometheusMetric, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PrometheusPollSpec.
func (in *PrometheusPollSpec) DeepCopy() *PrometheusPollSpec {
	if in == nil {
		return nil
	}
	out := new(PrometheusPollSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PrometheusPollStatus) DeepCopyInto(out *PrometheusPollStatus) {
	*out = *in
	in.MetricValuesProviderStatus.DeepCopyInto(&out.MetricValuesProviderStatus)
	if in.LastPollingTime != nil {
		in, out := &in.LastPollingTime, &out.LastPollingTime
		*out = (*in).DeepCopy()
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PrometheusPollStatus.
func (in *PrometheusPollStatus) DeepCopy() *PrometheusPollStatus {
	if in == nil {
		return nil
	}
	out := new(PrometheusPollStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RobustScalingNormalizer) DeepCopyInto(out *RobustScalingNormalizer) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RobustScalingNormalizer.
func (in *RobustScalingNormalizer) DeepCopy() *RobustScalingNormalizer {
	if in == nil {
		return nil
	}
	out := new(RobustScalingNormalizer)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *RobustScalingNormalizer) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RobustScalingNormalizerList) DeepCopyInto(out *RobustScalingNormalizerList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]RobustScalingNormalizer, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RobustScalingNormalizerList.
func (in *RobustScalingNormalizerList) DeepCopy() *RobustScalingNormalizerList {
	if in == nil {
		return nil
	}
	out := new(RobustScalingNormalizerList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *RobustScalingNormalizerList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RobustScalingNormalizerSpec) DeepCopyInto(out *RobustScalingNormalizerSpec) {
	*out = *in
	in.NormalizerSpec.DeepCopyInto(&out.NormalizerSpec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RobustScalingNormalizerSpec.
func (in *RobustScalingNormalizerSpec) DeepCopy() *RobustScalingNormalizerSpec {
	if in == nil {
		return nil
	}
	out := new(RobustScalingNormalizerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RobustScalingNormalizerStatus) DeepCopyInto(out *RobustScalingNormalizerStatus) {
	*out = *in
	in.MetricValuesProviderStatus.DeepCopyInto(&out.MetricValuesProviderStatus)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RobustScalingNormalizerStatus.
func (in *RobustScalingNormalizerStatus) DeepCopy() *RobustScalingNormalizerStatus {
	if in == nil {
		return nil
	}
	out := new(RobustScalingNormalizerStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecretTypeClusterShardManager) DeepCopyInto(out *SecretTypeClusterShardManager) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SecretTypeClusterShardManager.
func (in *SecretTypeClusterShardManager) DeepCopy() *SecretTypeClusterShardManager {
	if in == nil {
		return nil
	}
	out := new(SecretTypeClusterShardManager)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SecretTypeClusterShardManager) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecretTypeClusterShardManagerList) DeepCopyInto(out *SecretTypeClusterShardManagerList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SecretTypeClusterShardManager, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SecretTypeClusterShardManagerList.
func (in *SecretTypeClusterShardManagerList) DeepCopy() *SecretTypeClusterShardManagerList {
	if in == nil {
		return nil
	}
	out := new(SecretTypeClusterShardManagerList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SecretTypeClusterShardManagerList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecretTypeClusterShardManagerSpec) DeepCopyInto(out *SecretTypeClusterShardManagerSpec) {
	*out = *in
	in.ShardManagerSpec.DeepCopyInto(&out.ShardManagerSpec)
	if in.LabelSelector != nil {
		in, out := &in.LabelSelector, &out.LabelSelector
		*out = new(v1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SecretTypeClusterShardManagerSpec.
func (in *SecretTypeClusterShardManagerSpec) DeepCopy() *SecretTypeClusterShardManagerSpec {
	if in == nil {
		return nil
	}
	out := new(SecretTypeClusterShardManagerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecretTypeClusterShardManagerStatus) DeepCopyInto(out *SecretTypeClusterShardManagerStatus) {
	*out = *in
	in.ShardsProviderStatus.DeepCopyInto(&out.ShardsProviderStatus)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SecretTypeClusterShardManagerStatus.
func (in *SecretTypeClusterShardManagerStatus) DeepCopy() *SecretTypeClusterShardManagerStatus {
	if in == nil {
		return nil
	}
	out := new(SecretTypeClusterShardManagerStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WeightedPNormLoadIndex) DeepCopyInto(out *WeightedPNormLoadIndex) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WeightedPNormLoadIndex.
func (in *WeightedPNormLoadIndex) DeepCopy() *WeightedPNormLoadIndex {
	if in == nil {
		return nil
	}
	out := new(WeightedPNormLoadIndex)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WeightedPNormLoadIndex) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WeightedPNormLoadIndexList) DeepCopyInto(out *WeightedPNormLoadIndexList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]WeightedPNormLoadIndex, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WeightedPNormLoadIndexList.
func (in *WeightedPNormLoadIndexList) DeepCopy() *WeightedPNormLoadIndexList {
	if in == nil {
		return nil
	}
	out := new(WeightedPNormLoadIndexList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WeightedPNormLoadIndexList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WeightedPNormLoadIndexSpec) DeepCopyInto(out *WeightedPNormLoadIndexSpec) {
	*out = *in
	in.LoadIndexerSpec.DeepCopyInto(&out.LoadIndexerSpec)
	if in.Weights != nil {
		in, out := &in.Weights, &out.Weights
		*out = make([]WeightedPNormLoadIndexWeight, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WeightedPNormLoadIndexSpec.
func (in *WeightedPNormLoadIndexSpec) DeepCopy() *WeightedPNormLoadIndexSpec {
	if in == nil {
		return nil
	}
	out := new(WeightedPNormLoadIndexSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WeightedPNormLoadIndexStatus) DeepCopyInto(out *WeightedPNormLoadIndexStatus) {
	*out = *in
	in.LoadIndexProviderStatus.DeepCopyInto(&out.LoadIndexProviderStatus)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WeightedPNormLoadIndexStatus.
func (in *WeightedPNormLoadIndexStatus) DeepCopy() *WeightedPNormLoadIndexStatus {
	if in == nil {
		return nil
	}
	out := new(WeightedPNormLoadIndexStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WeightedPNormLoadIndexWeight) DeepCopyInto(out *WeightedPNormLoadIndexWeight) {
	*out = *in
	out.Weight = in.Weight.DeepCopy()
	if in.Negative != nil {
		in, out := &in.Negative, &out.Negative
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WeightedPNormLoadIndexWeight.
func (in *WeightedPNormLoadIndexWeight) DeepCopy() *WeightedPNormLoadIndexWeight {
	if in == nil {
		return nil
	}
	out := new(WeightedPNormLoadIndexWeight)
	in.DeepCopyInto(out)
	return out
}
