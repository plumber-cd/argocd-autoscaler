package harness

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Client is a small wrapper over controller-runtime client.Client.
// It defines additional methods that accepts ObjectContainer instead of client.Object.
type Client struct {
	client.Client
}

// CreateContainer wraps client.Client.Create method with ObjectContainer.
func (c *Client) CreateContainer(ctx context.Context, container *ObjectContainer[client.Object]) error {
	return c.Client.Create(ctx, container.Object())
}

// GetContainer wraps client.Client.Get method with ObjectContainer and ObjectKey.
func (c *Client) GetContainer(ctx context.Context, container *ObjectContainer[client.Object]) error {
	return c.Client.Get(ctx, container.ObjectKey(), container.Object())
}

// UpdateContainer wraps client.Client.Update method with ObjectContainer.
func (c *Client) UpdateContainer(ctx context.Context, container *ObjectContainer[client.Object]) error {
	return c.Client.Update(ctx, container.Object())
}

// StatusUpdateContainer wraps client.Client.Status().Update method with ObjectContainer.
func (c *Client) StatusUpdateContainer(ctx context.Context, container *ObjectContainer[client.Object]) error {
	return c.Client.Status().Update(ctx, container.Object())
}

// DeleteContainer wraps client.Client.Delete method with ObjectContainer.
func (c *Client) DeleteContainer(ctx context.Context, container *ObjectContainer[client.Object]) error {
	return c.Client.Delete(ctx, container.Object())
}
