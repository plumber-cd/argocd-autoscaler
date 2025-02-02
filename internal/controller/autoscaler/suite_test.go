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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/uuid"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	autoscalerv1alpha1 "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
	// +kubebuilder:scaffold:imports
)

type fakeGetFn func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
type fakeListFn func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
type fakeUpdateFn func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error
type fakeUpdateSubResourceFn func(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error
type fakeDeleteFn func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error

type fakeClient struct {
	client.Client
	getFunctions          map[reflect.Type]map[client.ObjectKey]fakeGetFn
	listFunctions         map[reflect.Type]fakeListFn
	updateFunctions       map[reflect.Type]map[client.ObjectKey]fakeUpdateFn
	statusUpdateFunctions map[reflect.Type]map[client.ObjectKey]fakeUpdateSubResourceFn
	deleteFn              map[reflect.Type]map[client.ObjectKey]fakeDeleteFn
}

type fakeStatusWriter struct {
	client.StatusWriter
	parent *fakeClient
}

func (f *fakeClient) WithGetFunction(obj client.Object, key client.ObjectKey, fn fakeGetFn) *fakeClient {
	if f.getFunctions == nil {
		f.getFunctions = make(map[reflect.Type]map[client.ObjectKey]fakeGetFn)
	}
	if f.getFunctions[reflect.TypeOf(obj)] == nil {
		f.getFunctions[reflect.TypeOf(obj)] = make(map[client.ObjectKey]fakeGetFn)
	}
	f.getFunctions[reflect.TypeOf(obj)][key] = fn
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

func (f *fakeClient) WithListFunction(list client.ObjectList, fn fakeListFn) *fakeClient {
	if f.listFunctions == nil {
		f.listFunctions = make(map[reflect.Type]fakeListFn)
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

func (f *fakeClient) WithUpdateFunction(obj client.Object, key client.ObjectKey, fn fakeUpdateFn) *fakeClient {
	if f.updateFunctions == nil {
		f.updateFunctions = make(map[reflect.Type]map[client.ObjectKey]fakeUpdateFn)
	}
	if f.updateFunctions[reflect.TypeOf(obj)] == nil {
		f.updateFunctions[reflect.TypeOf(obj)] = make(map[client.ObjectKey]fakeUpdateFn)
	}
	f.updateFunctions[reflect.TypeOf(obj)][key] = fn
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

func (f *fakeClient) WithStatusUpdateFunction(obj client.Object, key client.ObjectKey, fn fakeUpdateSubResourceFn) *fakeClient {
	if f.statusUpdateFunctions == nil {
		f.statusUpdateFunctions = make(map[reflect.Type]map[client.ObjectKey]fakeUpdateSubResourceFn)
	}
	if f.statusUpdateFunctions[reflect.TypeOf(obj)] == nil {
		f.statusUpdateFunctions[reflect.TypeOf(obj)] = make(map[client.ObjectKey]fakeUpdateSubResourceFn)
	}
	f.statusUpdateFunctions[reflect.TypeOf(obj)][key] = fn
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

func (f *fakeClient) WithDeleteFunction(obj client.Object, key client.ObjectKey, fn fakeDeleteFn) *fakeClient {
	if f.deleteFn == nil {
		f.deleteFn = make(map[reflect.Type]map[client.ObjectKey]fakeDeleteFn)
	}
	if f.deleteFn[reflect.TypeOf(obj)] == nil {
		f.deleteFn[reflect.TypeOf(obj)] = make(map[client.ObjectKey]fakeDeleteFn)
	}
	f.deleteFn[reflect.TypeOf(obj)][key] = fn
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

func newNamespace() (client.ObjectKey, types.NamespacedName) {
	namespaceName := client.ObjectKey{
		Name: "test-" + string(uuid.NewUUID()),
	}
	namespacedName := types.NamespacedName{
		Name:      "test",
		Namespace: namespaceName.Name,
	}
	return namespaceName, namespacedName
}

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	ctx       context.Context
	cancel    context.CancelFunc
	testEnv   *envtest.Environment
	cfg       *rest.Config
	k8sClient client.Client
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

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
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
