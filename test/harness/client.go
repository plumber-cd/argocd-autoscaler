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
