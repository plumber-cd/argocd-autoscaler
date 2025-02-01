package autoscaler

const (
	// StatusTypeAvailable represents the status of the resource reconciliation
	StatusTypeAvailable = "Available"
	// StatusTypeAvailableReasonInitialization indicates the resource is being initialized
	StatusTypeAvailableReasonInitialization = "Initialization"
	// StatusTypeReady represents the status of the resource
	StatusTypeReady = "Ready"
)
