package harness

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ScenarioSeedFn is a signature of a function that seeds a resource.
type ScenarioSeedFn[K client.Object] func() K

// ScenarioCleanClientFn is a signature of a function that returns a clean client.
type ScenarioCleanClientFn[K client.Object] func(*ScenarioRun[K])

// ScenarioHydrationFn is a signature of a function that hydrates a resource.
type ScenarioHydrationFn[K client.Object] func(*ScenarioRun[K])

// ScenarioFakeClientFn is a signature of a function that returns a fake client.
type ScenarioFakeClientFn[K client.Object] func(*ScenarioRun[K])

// ScenarioReconcilerFn is a signature of a function that returns a reconciler.
type ScenarioReconcilerFn[R reconcile.TypedReconciler[reconcile.Request]] func(client.Client) R

// ScenarioCheckFn is a signature of a function that checks the result of a scenario.
type ScenarioCheckFn[K client.Object] func(*ScenarioRun[K])

// ScenarioRegisterCallback is a signature of a callback function to register a scenario in collector.
type ScenarioRegisterCallback[K client.Object] func(*ScenarioWithCheck[K])

// ScenarioItFn is a function that is called from ginkgo to run a scenario.
type ScenarioItFn func(context.Context, *Client, *ObjectContainer[*corev1.Namespace])

// ScenarioCollector is a collector to instruct ginkgo.
type ScenarioCollector[K client.Object, R reconcile.TypedReconciler[reconcile.Request]] struct {
	reconcilerFn ScenarioReconcilerFn[R]
	checks       []*ScenarioWithCheck[K]
}

// ScenarioTemplate is a template for a scenario.
// Container may be initialized with data some data, but for the most part - it will be hydrated and created in k8s later.
type ScenarioTemplate[K client.Object] struct {
	templateName string
	seedFn       ScenarioSeedFn[K]
}

// ScenarioWithHydration is a scenario instantiated from a template with a set of hydration functions.
// Hydration functions are used to prepare the resource for the test, and create it in the cluster.
// Hydration functions will be using a clean client.
type ScenarioWithHydration[K client.Object] struct {
	*ScenarioTemplate[K]
	hydrationFn        ScenarioHydrationFn[K]
	hydrationPatchesFn []ScenarioHydrationFn[K]
}

// ScenarioWithFakeClient is a ScenarioWithHydration with ScenarioFakeClientFn.
type ScenarioWithFakeClient[K client.Object] struct {
	*ScenarioWithHydration[K]
	fakeClientFn        ScenarioFakeClientFn[K]
	fakeClientPatchesFn []ScenarioFakeClientFn[K]
}

// ScenarioWithCheck is a ScenarioWithFakeClient with ScenarioCheckFn.
type ScenarioWithCheck[K client.Object] struct {
	*ScenarioWithFakeClient[K]
	checkName string
	checkFn   []ScenarioCheckFn[K]
}

// ScenarioRun is a full assembly of a scenario, representing its state at various stages of the run.
type ScenarioRun[K client.Object] struct {
	Context         context.Context
	Client          *Client
	Namespace       *ObjectContainer[*corev1.Namespace]
	FakeClient      *FakeClient
	SeedObject      K
	Container       *ObjectContainer[K]
	ReconcileResult reconcile.Result
	ReconcileError  error
}

// ScenarioIt is a wrapper for ginkgo.
type ScenarioIt struct {
	TemplateName string
	CheckName    string
	It           ScenarioItFn
}

// NewScenarioCollector creates a new scenario collector.
func NewScenarioCollector[K client.Object, R reconcile.TypedReconciler[reconcile.Request]](
	reconcilerFn ScenarioReconcilerFn[R],
) *ScenarioCollector[K, R] {
	return &ScenarioCollector[K, R]{
		reconcilerFn: reconcilerFn,
	}
}

// NewScenarioTemplate creates a template of a new scenario.
// The object, must be initialized with metav1.ObjectMeta.Name.
// The object may or may not be initialized with additional data.
func NewScenarioTemplate[K client.Object](
	name string,
	seedFn ScenarioSeedFn[K],
) *ScenarioTemplate[K] {
	return &ScenarioTemplate[K]{
		templateName: name,
		seedFn:       seedFn,
	}
}

// Hydrate hydrates the scenario with a hydration function.
func (s *ScenarioTemplate[K]) Hydrate(
	hydrationFn ScenarioHydrationFn[K],
) *ScenarioWithHydration[K] {
	return &ScenarioWithHydration[K]{
		ScenarioTemplate:   s,
		hydrationFn:        hydrationFn,
		hydrationPatchesFn: []ScenarioHydrationFn[K]{},
	}
}

// HydrateByClientCreate hydrates the scenario with a default hydration function that creates the object as-is in the cluster in the test namespace.
func (s *ScenarioTemplate[K]) HydrateByClientCreate() *ScenarioWithHydration[K] {
	fn := func(run *ScenarioRun[K]) {
		container := NewObjectContainer(run.SeedObject)
		Expect(
			run.Client.CreateContainer(run.Context, container.ClientObject()),
		).To(Succeed())
		Expect(
			run.Client.GetContainer(run.Context, container.ClientObject()),
		).To(Succeed())
		run.Container = container
	}
	return s.Hydrate(fn)
}

// HydrateByCreateContainer creates a hydrated scenario without creating k8s resources.
// Useful for scenarios that checks against non-existent resources.
func (s *ScenarioTemplate[K]) HydrateByCreateContainer() *ScenarioWithHydration[K] {
	fn := func(run *ScenarioRun[K]) {
		container := NewObjectContainer(run.SeedObject)
		run.Container = container
	}
	return s.Hydrate(fn)
}

// Hydrate hydrates the scenario with additional hydration function.
func (s *ScenarioWithHydration[K]) Hydrate(
	hydrationFn ScenarioHydrationFn[K],
) *ScenarioWithHydration[K] {
	s.hydrationPatchesFn = append(s.hydrationPatchesFn, hydrationFn)
	return s
}

// DeHydrate returns a scenario ScenarioTemplate ready to be re-hydrated with something else.
func (s *ScenarioWithHydration[K]) DeHydrate() *ScenarioTemplate[K] {
	return &ScenarioTemplate[K]{
		templateName: s.templateName,
		seedFn:       s.seedFn,
	}
}

// ResetHydrationPatches creates a new clone of this ScenarioWithHydration but without hydration patches.
func (s *ScenarioWithHydration[K]) ResetHydrationPatches() *ScenarioWithHydration[K] {
	return &ScenarioWithHydration[K]{
		ScenarioTemplate: s.DeHydrate(),
		hydrationFn:      s.hydrationFn,
	}
}

// WithFakeClient creates a scenario that can be executed and ready for checks.
// The fakeClientFn will be used to create a fake client for the scenario.
func (s *ScenarioWithHydration[K]) WithFakeClient(
	fakeClientFn ScenarioFakeClientFn[K],
) *ScenarioWithFakeClient[K] {
	return &ScenarioWithFakeClient[K]{
		ScenarioWithHydration: s,
		fakeClientFn:          fakeClientFn,
	}
}

// WithCleanClient creates a scenario that can be executed and ready for checks.
// It will be using clean client as-is, meaning that no mocks are necessary for this scenario.
func (s *ScenarioWithHydration[K]) WithCleanClient() *ScenarioWithFakeClient[K] {
	fn := func(run *ScenarioRun[K]) {
		run.FakeClient = &FakeClient{Client: run.Client}
	}
	return s.WithFakeClient(fn)
}

// WithFakeClientExtension extends the fake client with additional mocks.
// If parent fake client was not set - it will be created with a clean client.
func (s *ScenarioWithFakeClient[K]) WithFakeClientExtension(
	fakeClientFn ScenarioFakeClientFn[K],
) *ScenarioWithFakeClient[K] {
	s.fakeClientPatchesFn = append(s.fakeClientPatchesFn, fakeClientFn)
	return s
}

// WithFakeClientThatFailsToGetResource creates a scenario with a fake client that is a copy of the parent client.
// The copy of the client will have additional mock to simulate failure to get the resource.
// If parent fake client was not set - it will be created with a clean client.
func (s *ScenarioWithFakeClient[K]) WithFakeClientThatFailsToGetResource() *ScenarioWithFakeClient[K] {
	fn := func(run *ScenarioRun[K]) {
		fakeClient := &FakeClient{
			Client: run.FakeClient,
		}
		fakeClient.
			WithGetFunction(run.Container.ClientObject(),
				func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return errors.NewBadRequest("fake error getting resource")
				},
			)
		run.FakeClient = fakeClient
	}
	return s.WithFakeClientExtension(fn)
}

// WithFakeClientThatFailsToUpdateStatus creates a scenario with a client that is a copy of the parent client.
// The copy of the client will have additional mock to simulate failure to update the status.
// If parent fake client was not set - it will be created with a clean client.
func (s *ScenarioWithFakeClient[K]) WithFakeClientThatFailsToUpdateStatus() *ScenarioWithFakeClient[K] {
	fn := func(run *ScenarioRun[K]) {
		fakeClient := &FakeClient{
			Client: run.FakeClient,
		}
		fakeClient.
			WithStatusUpdateFunction(run.Container.ClientObject(),
				func(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
					return errors.NewBadRequest("fake error updating status")
				},
			)
		run.FakeClient = fakeClient
	}
	return s.WithFakeClientExtension(fn)
}

// RemoveClient returns HydratedScenario ready to be re-hydrated with something else.
func (s *ScenarioWithFakeClient[K]) RemoveClient() *ScenarioWithHydration[K] {
	return &ScenarioWithHydration[K]{
		ScenarioTemplate:   s.ScenarioWithHydration.DeHydrate(),
		hydrationFn:        s.ScenarioWithHydration.hydrationFn,
		hydrationPatchesFn: s.ScenarioWithHydration.hydrationPatchesFn,
	}
}

// ResetClientPatches returns a copy of ScenarioWithFakeClient but without the patches.
func (s *ScenarioWithFakeClient[K]) ResetClientPatches() *ScenarioWithFakeClient[K] {
	return &ScenarioWithFakeClient[K]{
		ScenarioWithHydration: s.RemoveClient(),
		fakeClientFn:          s.fakeClientFn,
	}
}

// WithCheck creates a ScenarioWithCheck.
func (s *ScenarioWithFakeClient[K]) WithCheck(
	checkName string,
	checkFn ScenarioCheckFn[K],
) *ScenarioWithCheck[K] {
	return &ScenarioWithCheck[K]{
		ScenarioWithFakeClient: s,
		checkName:              checkName,
		checkFn:                []ScenarioCheckFn[K]{checkFn},
	}
}

// AdHocCheck allows to branch out a scenario for additional checks without the intention to extending upon it.
// Creates a copy of the scenario but returns original scenario.
func (s *ScenarioWithFakeClient[K]) AdHocCheck(branch func(*ScenarioWithFakeClient[K])) *ScenarioWithFakeClient[K] {
	clone := &ScenarioWithFakeClient[K]{
		ScenarioWithHydration: s.RemoveClient(),
		fakeClientFn:          s.fakeClientFn,
		fakeClientPatchesFn:   s.fakeClientPatchesFn,
	}
	branch(clone)
	return s
}

// AdHocCheckResourceNotFound is a shortcut function that uses scenario client and a set of checks that are expecting clean exit with no re-queue.
// Does not modify the state, acts ad ad-hoc check with ability to continue scenario.
func (s *ScenarioWithFakeClient[K]) AdHocCheckResourceNotFound(callback ScenarioRegisterCallback[K]) *ScenarioWithFakeClient[K] {
	clone := &ScenarioWithFakeClient[K]{
		ScenarioWithHydration: s.RemoveClient(),
		fakeClientFn:          s.fakeClientFn,
		fakeClientPatchesFn:   s.fakeClientPatchesFn,
	}
	callback(
		clone.
			WithCheck("should successfully exit", func(run *ScenarioRun[K]) {
				Expect(run.ReconcileError).NotTo(HaveOccurred())
				Expect(run.ReconcileResult.RequeueAfter).To(Equal(time.Duration(0)))
				Expect(run.ReconcileResult.Requeue).To(BeFalse())
			}),
	)
	return s
}

// AdHocCheckFailureToGetResource is a shortcut function that uses a WithClientFromParentThatFailsToGetResource() and a set of checks that are expecting a corresponding error from it.
// Does not modify the state, acts ad ad-hoc check with ability to continue scenario.
func (s *ScenarioWithFakeClient[K]) AdHocCheckFailureToGetResource(callback ScenarioRegisterCallback[K]) *ScenarioWithFakeClient[K] {
	clone := &ScenarioWithFakeClient[K]{
		ScenarioWithHydration: s.RemoveClient(),
		fakeClientFn:          s.fakeClientFn,
		fakeClientPatchesFn:   s.fakeClientPatchesFn,
	}
	callback(
		clone.
			WithFakeClientThatFailsToGetResource().
			WithCheck("should handle errors during getting resource", func(run *ScenarioRun[K]) {
				Expect(run.ReconcileError).To(HaveOccurred())
				Expect(run.ReconcileError.Error()).To(Equal("fake error getting resource"))
				Expect(run.ReconcileResult.RequeueAfter).To(Equal(time.Second))
				Expect(run.ReconcileResult.Requeue).To(BeFalse())
			}),
	)
	return s
}

// AdHocCheckFailureToUpdateStatus is a shortcut function that uses a WithClientFromParentThatFailsToUpdateStatus() and a set of check that are expecting a corresponding error from it.
// Does not modify the state, acts ad ad-hoc check with ability to continue scenario.
func (s *ScenarioWithFakeClient[K]) AdHocCheckFailureToUpdateStatus(callback ScenarioRegisterCallback[K]) *ScenarioWithFakeClient[K] {
	clone := &ScenarioWithFakeClient[K]{
		ScenarioWithHydration: s.RemoveClient(),
		fakeClientFn:          s.fakeClientFn,
		fakeClientPatchesFn:   s.fakeClientPatchesFn,
	}
	callback(
		clone.
			WithFakeClientThatFailsToUpdateStatus().
			WithCheck("should handle errors during updating status on a resource", func(run *ScenarioRun[K]) {
				Expect(run.ReconcileError).To(HaveOccurred())
				Expect(run.ReconcileError.Error()).To(Equal("fake error updating status"))
				Expect(run.ReconcileResult.RequeueAfter).To(Equal(time.Second))
			}),
	)
	return s
}

// WithCheck adds a check to the scenario.
func (s *ScenarioWithCheck[K]) WithCheck(
	checkFn ScenarioCheckFn[K],
) *ScenarioWithCheck[K] {
	s.checkFn = append(s.checkFn, checkFn)
	return s
}

// Register registers a scenario in the by calling a collector callback.
// Returns a ScenarioWithFakeClient ready for another set of checks with additional mocks.
func (s *ScenarioWithCheck[K]) Register(callback ScenarioRegisterCallback[K]) *ScenarioWithFakeClient[K] {
	callback(s)
	return &ScenarioWithFakeClient[K]{
		ScenarioWithHydration: s.RemoveClient(),
		fakeClientFn:          s.fakeClientFn,
		fakeClientPatchesFn:   s.fakeClientPatchesFn,
	}
}

// Registrator is a callback function to register a scenario in the collector.
func (c *ScenarioCollector[K, R]) Registrator(check *ScenarioWithCheck[K]) {
	c.checks = append(c.checks, check)
}

func (c *ScenarioCollector[K, R]) All() []ScenarioIt {
	its := []ScenarioIt{}
	for _, check := range c.checks {
		its = append(its, ScenarioIt{
			TemplateName: check.templateName,
			CheckName:    check.checkName,
			It: func(ctx context.Context, client *Client, ns *ObjectContainer[*corev1.Namespace]) {
				By("Creating scenario run")
				run := &ScenarioRun[K]{
					Context:   ctx,
					Client:    client,
					Namespace: ns,
				}
				By("Seeding object")
				run.SeedObject = check.seedFn()
				run.SeedObject.SetNamespace(run.Namespace.Object().Name)
				By("Hydrating")
				check.hydrationFn(run)
				for _, fn := range check.hydrationPatchesFn {
					fn(run)
				}
				By("Creating fake client")
				check.fakeClientFn(run)
				for _, fn := range check.fakeClientPatchesFn {
					fn(run)
				}
				By("Creating reconciler")
				reconciler := c.reconcilerFn(run.FakeClient)
				By("Reconciling")
				result, err := reconciler.Reconcile(run.Context, reconcile.Request{
					NamespacedName: run.Container.NamespacedName(),
				})
				run.ReconcileResult = result
				run.ReconcileError = err
				By("Checking")
				for _, checkFn := range check.checkFn {
					checkFn(run)
				}
				// By("Simulating error")
				// Fail("Simulating error")
			},
		})
	}
	return its
}
