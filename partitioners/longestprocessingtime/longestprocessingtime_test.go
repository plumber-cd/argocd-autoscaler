package longestprocessingtime

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "LongestProcessingTimePartitioner Suite")
}

var _ = Describe("LongestProcessingTimePartitioner", func() {
	Context("when unsorted list of shards", func() {
		// nolint:dupl
		It("should sort", func() {
			shards := []common.LoadIndex{
				{Shard: common.Shard{Name: "shard1"}, Value: resource.MustParse("5")},
				{Shard: common.Shard{Name: "shard2"}, Value: resource.MustParse("3")},
				{Shard: common.Shard{Name: "shard3"}, Value: resource.MustParse("8")},
				{Shard: common.Shard{Name: "shard4"}, Value: resource.MustParse("6")},
				{Shard: common.Shard{Name: "shard5"}, Value: resource.MustParse("7")},
			}
			partitioner := PartitionerImpl{}
			sortedShards, bucketSize := partitioner.Sort(shards)
			Expect(bucketSize).To(Equal(float64(8)))
			Expect(sortedShards).To(HaveLen(len(shards)))
			Expect(sortedShards[0].Shard.Name).To(Equal("shard3"))
			Expect(sortedShards[1].Shard.Name).To(Equal("shard5"))
			Expect(sortedShards[2].Shard.Name).To(Equal("shard4"))
			Expect(sortedShards[3].Shard.Name).To(Equal("shard1"))
			Expect(sortedShards[4].Shard.Name).To(Equal("shard2"))
		})

		// nolint:dupl
		It("should put in-cluster first", func() {
			shards := []common.LoadIndex{
				{Shard: common.Shard{Name: "shard1"}, Value: resource.MustParse("5")},
				{Shard: common.Shard{Name: "shard2"}, Value: resource.MustParse("3")},
				{Shard: common.Shard{Name: "shard3"}, Value: resource.MustParse("8")},
				{Shard: common.Shard{Name: "in-cluster"}, Value: resource.MustParse("6")},
				{Shard: common.Shard{Name: "shard5"}, Value: resource.MustParse("7")},
			}
			partitioner := PartitionerImpl{}
			sortedShards, bucketSize := partitioner.Sort(shards)
			Expect(bucketSize).To(Equal(float64(8)))
			Expect(sortedShards).To(HaveLen(len(shards)))
			Expect(sortedShards[0].Shard.Name).To(Equal("in-cluster"))
			Expect(sortedShards[1].Shard.Name).To(Equal("shard3"))
			Expect(sortedShards[2].Shard.Name).To(Equal("shard5"))
			Expect(sortedShards[3].Shard.Name).To(Equal("shard1"))
			Expect(sortedShards[4].Shard.Name).To(Equal("shard2"))
		})
	})

	Context("when partitioning", func() {
		It("should partition", func() {
			shards := []common.LoadIndex{
				{Shard: common.Shard{Name: "shard1"}, Value: resource.MustParse("5")},
				{Shard: common.Shard{Name: "shard2"}, Value: resource.MustParse("3")},
				{Shard: common.Shard{Name: "shard3"}, Value: resource.MustParse("10")},
				{Shard: common.Shard{Name: "in-cluster"}, Value: resource.MustParse("6")},
				{Shard: common.Shard{Name: "shard5"}, Value: resource.MustParse("7")},
			}
			partitioner := PartitionerImpl{}
			replicas, err := partitioner.Partition(context.TODO(), shards)
			Expect(err).ToNot(HaveOccurred())
			Expect(replicas).To(HaveLen(4))

			Expect(replicas[0].ID).To(Equal(int32(0)))
			Expect(replicas[0].LoadIndexes).To(HaveLen(1))
			Expect(replicas[0].LoadIndexes[0].Shard.Name).To(Equal("in-cluster"))
			Expect(replicas[0].TotalLoad.AsApproximateFloat64()).To(Equal(float64(6)))
			Expect(replicas[0].TotalLoadDisplayValue).To(Equal("6"))

			Expect(replicas[1].ID).To(Equal(int32(1)))
			Expect(replicas[1].LoadIndexes).To(HaveLen(1))
			Expect(replicas[1].LoadIndexes[0].Shard.Name).To(Equal("shard3"))
			Expect(replicas[1].TotalLoad.AsApproximateFloat64()).To(Equal(float64(10)))
			Expect(replicas[1].TotalLoadDisplayValue).To(Equal("10"))

			Expect(replicas[2].ID).To(Equal(int32(2)))
			Expect(replicas[2].LoadIndexes).To(HaveLen(1))
			Expect(replicas[2].LoadIndexes[0].Shard.Name).To(Equal("shard5"))
			Expect(replicas[2].TotalLoad.AsApproximateFloat64()).To(Equal(float64(7)))
			Expect(replicas[2].TotalLoadDisplayValue).To(Equal("7"))

			Expect(replicas[3].ID).To(Equal(int32(3)))
			Expect(replicas[3].LoadIndexes).To(HaveLen(2))
			Expect(replicas[3].LoadIndexes[0].Shard.Name).To(Equal("shard1"))
			Expect(replicas[3].LoadIndexes[1].Shard.Name).To(Equal("shard2"))
			Expect(replicas[3].TotalLoad.AsApproximateFloat64()).To(Equal(float64(8)))
			Expect(replicas[3].TotalLoadDisplayValue).To(Equal("8"))
		})
	})
})
