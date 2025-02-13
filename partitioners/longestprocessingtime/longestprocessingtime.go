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

package longestprocessingtime

import (
	"context"
	"sort"
	"strconv"

	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
)

type Partitioner interface {
	Partition(ctx context.Context, shards []common.LoadIndex) (common.ReplicaList, error)
}

type PartitionerImpl struct{}

func (r *PartitionerImpl) Partition(ctx context.Context,
	shards []common.LoadIndex) (common.ReplicaList, error) {

	log := log.FromContext(ctx)

	replicas := common.ReplicaList{}

	if len(shards) == 0 {
		return replicas, nil
	}

	sort.Sort(common.LoadIndexesDesc(shards))
	bucketSize := shards[0].Value.AsApproximateFloat64()

	replicaCount := int32(0)
	for _, shard := range shards {
		// Find the replica with the least current load.
		minLoad := float64(0)
		selectedReplicaIndex := -1

		for i, replica := range replicas {
			if selectedReplicaIndex == -1 || replica.TotalLoad.AsApproximateFloat64() < minLoad {
				if replica.TotalLoad.AsApproximateFloat64()+shard.Value.AsApproximateFloat64() < bucketSize {
					selectedReplicaIndex = i
					minLoad = replica.TotalLoad.AsApproximateFloat64()
				}
			}
		}

		if selectedReplicaIndex < 0 {
			// Create a new replica for this shard.
			replicaCount++
			newReplica := common.Replica{
				ID:                    strconv.FormatInt(int64(replicaCount-1), 10),
				LoadIndexes:           []common.LoadIndex{shard},
				TotalLoad:             shard.Value,
				TotalLoadDisplayValue: strconv.FormatFloat(shard.Value.AsApproximateFloat64(), 'f', -1, 64),
			}
			replicas = append(replicas, newReplica)
		} else {
			// Assign shard to the selected replica.
			replicas[selectedReplicaIndex].LoadIndexes = append(replicas[selectedReplicaIndex].LoadIndexes, shard)
			totalLoad := replicas[selectedReplicaIndex].TotalLoad.AsApproximateFloat64()
			totalLoad += shard.Value.AsApproximateFloat64()
			totalLoadAsString := strconv.FormatFloat(totalLoad, 'f', -1, 64)
			replicas[selectedReplicaIndex].TotalLoadDisplayValue = totalLoadAsString
			totalLoadAsResource, err := resource.ParseQuantity(totalLoadAsString)
			if err != nil {
				log.Error(err, "Failed to parse total load as resource",
					"shard", shard.Shard.ID, "totalLoad", totalLoadAsResource)
				return nil, err
			}
			replicas[selectedReplicaIndex].TotalLoad = totalLoadAsResource
		}
	}

	return replicas, nil
}
