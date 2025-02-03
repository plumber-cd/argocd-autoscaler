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
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/uuid"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	autoscalerv1alpha1 "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
	// +kubebuilder:scaffold:imports
)

type objectContainer[K client.Object] struct {
	NamespacedName types.NamespacedName
	ObjectKey      client.ObjectKey
	Object         K
}

func NewObjectContainer[K client.Object](obj K, prep ...func(*objectContainer[K])) *objectContainer[K] {
	container := &objectContainer[K]{
		NamespacedName: types.NamespacedName{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		},
		ObjectKey: client.ObjectKey{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		},
		Object: obj,
	}
	for _, p := range prep {
		p(container)
	}
	return container
}

func (c *objectContainer[K]) Generic() *objectContainer[client.Object] {
	return &objectContainer[client.Object]{
		NamespacedName: c.NamespacedName,
		ObjectKey:      c.ObjectKey,
		Object:         c.Object,
	}
}

type getFn func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
type listFn func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
type updateFn func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error
type updateSubResourceFn func(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error
type deleteFn func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error

type fakeClient struct {
	ksClient
	getFunctions          map[reflect.Type]map[client.ObjectKey]getFn
	listFunctions         map[reflect.Type]listFn
	updateFunctions       map[reflect.Type]map[client.ObjectKey]updateFn
	statusUpdateFunctions map[reflect.Type]map[client.ObjectKey]updateSubResourceFn
	deleteFn              map[reflect.Type]map[client.ObjectKey]deleteFn
}

type fakeStatusWriter struct {
	client.StatusWriter
	parent *fakeClient
}

func (f *fakeClient) WithGetFunction(container *objectContainer[client.Object], fn getFn) *fakeClient {
	if f.getFunctions == nil {
		f.getFunctions = make(map[reflect.Type]map[client.ObjectKey]getFn)
	}
	if f.getFunctions[reflect.TypeOf(container.Object)] == nil {
		f.getFunctions[reflect.TypeOf(container.Object)] = make(map[client.ObjectKey]getFn)
	}
	f.getFunctions[reflect.TypeOf(container.Object)][container.ObjectKey] = fn
	return f
}

func (f *fakeClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if f.getFunctions != nil {
		if functions, ok := f.getFunctions[reflect.TypeOf(obj)]; ok {
			if fn, ok := functions[key]; ok {
				return fn(ctx, key, obj, opts...)
			}
		}
	}
	return f.Client.Get(ctx, key, obj, opts...)
}

func (f *fakeClient) WithListFunction(list client.ObjectList, fn listFn) *fakeClient {
	if f.listFunctions == nil {
		f.listFunctions = make(map[reflect.Type]listFn)
	}
	f.listFunctions[reflect.TypeOf(list)] = fn
	return f
}

func (f *fakeClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if f.listFunctions != nil {
		if fn, ok := f.listFunctions[reflect.TypeOf(list)]; ok {
			return fn(ctx, list, opts...)
		}
	}
	return f.Client.List(ctx, list, opts...)
}

func (f *fakeClient) WithUpdateFunction(container *objectContainer[client.Object], fn updateFn) *fakeClient {
	if f.updateFunctions == nil {
		f.updateFunctions = make(map[reflect.Type]map[client.ObjectKey]updateFn)
	}
	if f.updateFunctions[reflect.TypeOf(container.Object)] == nil {
		f.updateFunctions[reflect.TypeOf(container.Object)] = make(map[client.ObjectKey]updateFn)
	}
	f.updateFunctions[reflect.TypeOf(container.Object)][container.ObjectKey] = fn
	return f
}

func (f *fakeClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if f.updateFunctions != nil {
		if functions, ok := f.updateFunctions[reflect.TypeOf(obj)]; ok {
			if fn, ok := functions[client.ObjectKeyFromObject(obj)]; ok {
				return fn(ctx, obj, opts...)
			}
		}
	}
	return f.Client.Update(ctx, obj, opts...)
}

func (f *fakeClient) Status() client.StatusWriter {
	return &fakeStatusWriter{
		StatusWriter: f.Client.Status(),
		parent:       f,
	}
}

func (f *fakeClient) WithStatusUpdateFunction(container *objectContainer[client.Object], fn updateSubResourceFn) *fakeClient {
	if f.statusUpdateFunctions == nil {
		f.statusUpdateFunctions = make(map[reflect.Type]map[client.ObjectKey]updateSubResourceFn)
	}
	if f.statusUpdateFunctions[reflect.TypeOf(container.Object)] == nil {
		f.statusUpdateFunctions[reflect.TypeOf(container.Object)] = make(map[client.ObjectKey]updateSubResourceFn)
	}
	f.statusUpdateFunctions[reflect.TypeOf(container.Object)][container.ObjectKey] = fn
	return f
}

func (s *fakeStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	if s.parent.statusUpdateFunctions != nil {
		if functions, ok := s.parent.statusUpdateFunctions[reflect.TypeOf(obj)]; ok {
			if fn, ok := functions[client.ObjectKeyFromObject(obj)]; ok {
				return fn(ctx, obj, opts...)
			}
		}
	}
	return s.StatusWriter.Update(ctx, obj, opts...)
}

func (f *fakeClient) WithDeleteFunction(container *objectContainer[client.Object], fn deleteFn) *fakeClient {
	if f.deleteFn == nil {
		f.deleteFn = make(map[reflect.Type]map[client.ObjectKey]deleteFn)
	}
	if f.deleteFn[reflect.TypeOf(container.Object)] == nil {
		f.deleteFn[reflect.TypeOf(container.Object)] = make(map[client.ObjectKey]deleteFn)
	}
	f.deleteFn[reflect.TypeOf(container.Object)][container.ObjectKey] = fn
	return f
}

func (f *fakeClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if f.deleteFn != nil {
		if functions, ok := f.deleteFn[reflect.TypeOf(obj)]; ok {
			if fn, ok := functions[client.ObjectKeyFromObject(obj)]; ok {
				return fn(ctx, obj, opts...)
			}
		}
	}
	return f.Client.Delete(ctx, obj, opts...)
}

type ksClient struct {
	client.Client
}

func (c *ksClient) create(ctx context.Context, container *objectContainer[client.Object]) error {
	return c.Client.Create(ctx, container.Object)
}

func (c *ksClient) get(ctx context.Context, container *objectContainer[client.Object]) error {
	return c.Client.Get(ctx, container.ObjectKey, container.Object)
}

func (c *ksClient) update(ctx context.Context, container *objectContainer[client.Object]) error {
	return c.Client.Update(ctx, container.Object)
}

func (c *ksClient) statusUpdate(ctx context.Context, container *objectContainer[client.Object]) error {
	return c.Client.Status().Update(ctx, container.Object)
}

func (c *ksClient) delete(ctx context.Context, container *objectContainer[client.Object]) error {
	return c.Client.Delete(ctx, container.Object)
}

func newNamespaceWithRandomName() *objectContainer[*corev1.Namespace] {
	name := "test-" + string(uuid.NewUUID())
	return NewObjectContainer(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: name,
		},
	})
}

func CheckExitingOnNonExistingResource[R reconcile.TypedReconciler[reconcile.Request]](
	r func() R,
) {
	controllerReconciler := r()
	result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "non-existing-resource",
			Namespace: "default",
		},
	})

	Expect(err).NotTo(HaveOccurred())
	Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
	Expect(result.Requeue).To(BeFalse())
}

func CheckFailureToGetResource[K client.Object, R reconcile.TypedReconciler[reconcile.Request]](
	container *objectContainer[K],
	r func(*fakeClient) R,
) {
	fakeClient := &fakeClient{
		ksClient: k8sClient,
	}
	fakeClient.
		WithGetFunction(container.Generic(),
			func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				return errors.NewBadRequest("fake error getting resource")
			},
		)

	controllerReconciler := r(fakeClient)
	result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: container.NamespacedName,
	})

	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(Equal("fake error getting resource"))
	Expect(result.RequeueAfter).To(Equal(time.Second))
	Expect(result.Requeue).To(BeFalse())
}

func CheckFailureToUpdateStatus[K client.Object, R reconcile.TypedReconciler[reconcile.Request]](
	container *objectContainer[K],
	r func(*fakeClient) R,
) {
	fakeClient := &fakeClient{
		ksClient: k8sClient,
	}
	fakeClient.
		WithStatusUpdateFunction(container.Generic(),
			func(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
				return errors.NewBadRequest("fake error updating status")
			},
		)

	controllerReconciler := r(fakeClient)
	result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: container.NamespacedName,
	})

	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(Equal("fake error updating status"))
	Expect(result.RequeueAfter).To(Equal(time.Second))
}

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	ctx       context.Context
	cancel    context.CancelFunc
	testEnv   *envtest.Environment
	cfg       *rest.Config
	k8sClient ksClient
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	var err error
	err = autoscalerv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	// Retrieve the first found binary directory to allow running tests from IDEs
	if getFirstFoundEnvTestBinaryDir() != "" {
		testEnv.BinaryAssetsDirectory = getFirstFoundEnvTestBinaryDir()
	}

	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	_k8sClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(_k8sClient).NotTo(BeNil())

	k8sClient = ksClient{Client: _k8sClient}
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

// getFirstFoundEnvTestBinaryDir locates the first binary in the specified path.
// ENVTEST-based tests depend on specific binaries, usually located in paths set by
// controller-runtime. When running tests directly (e.g., via an IDE) without using
// Makefile targets, the 'BinaryAssetsDirectory' must be explicitly configured.
//
// This function streamlines the process by finding the required binaries, similar to
// setting the 'KUBEBUILDER_ASSETS' environment variable. To ensure the binaries are
// properly set up, run 'make setup-envtest' beforehand.
func getFirstFoundEnvTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		logf.Log.Error(err, "Failed to read directory", "path", basePath)
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}
	return ""
}
