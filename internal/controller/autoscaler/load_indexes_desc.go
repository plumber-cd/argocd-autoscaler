package autoscaler

import autoscalerv1alpha1 "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"

// LoadIndexesDesc implements sort.Interface for []LoadIndex based on the Value field in descending order.
type LoadIndexesDesc []autoscalerv1alpha1.LoadIndex

func (s LoadIndexesDesc) Len() int {
	return len(s)
}

func (s LoadIndexesDesc) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s LoadIndexesDesc) Less(i, j int) bool {
	return s[i].Value.AsApproximateFloat64() > s[j].Value.AsApproximateFloat64()
}
