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

	"k8s.io/apimachinery/pkg/util/uuid"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateNamespaceWithRandomName creates a new namespace with a random name, enclosed in an ObjectContainer.
func CreateNamespaceWithRandomName(ctx context.Context, client *Client) *ObjectContainer[*corev1.Namespace] {
	By("Creating a test namespace")
	name := "test-" + string(uuid.NewUUID())
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: name,
		},
	}
	container := NewObjectContainer(client.Scheme(), namespace)
	Expect(client.CreateContainer(ctx, container.ClientObject())).To(Succeed())
	Expect(client.GetContainer(ctx, container.ClientObject())).To(Succeed())
	return container
}

func DeleteNamespace(ctx context.Context, client *Client, container *ObjectContainer[*corev1.Namespace]) {
	By("Deleting the test namespace")
	Expect(client.GetContainer(ctx, container.ClientObject())).NotTo(HaveOccurred())
	Expect(client.DeleteContainer(ctx, container.ClientObject())).To(Succeed())
}
