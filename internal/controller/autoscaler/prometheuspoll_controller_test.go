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

package autoscaler

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	autoscalerv1alpha1 "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
)

var _ = Describe("PrometheusPoll Controller", func() {
	Context("When reconciling a resource", func() {
		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      "test",
			Namespace: "test-" + string(uuid.NewUUID()),
		}
		prometheuspoll := &autoscalerv1alpha1.PrometheusPoll{}

		BeforeEach(func() {
			By("Creating resources")
			Skip("Not implemented yet")
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: typeNamespacedName.Namespace,
				},
			}
			Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
			shardManager := &autoscalerv1alpha1.SecretTypeClusterShardManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      typeNamespacedName.Name,
					Namespace: typeNamespacedName.Namespace,
				},
				Status: autoscalerv1alpha1.SecretTypeClusterShardManagerStatus{},
			}
			Expect(k8sClient.Create(ctx, shardManager)).To(Succeed())
			resource := &autoscalerv1alpha1.PrometheusPoll{
				ObjectMeta: metav1.ObjectMeta{
					Name:      typeNamespacedName.Name,
					Namespace: typeNamespacedName.Namespace,
				},
				// TODO(user): Specify other spec details if needed.
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			Expect(k8sClient.Get(ctx, typeNamespacedName, prometheuspoll)).To(Succeed())
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &autoscalerv1alpha1.PrometheusPoll{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance PrometheusPoll")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &PrometheusPollReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
