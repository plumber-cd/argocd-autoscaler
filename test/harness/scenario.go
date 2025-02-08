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

// ScenarioClientFn is a signature of a function that adds a Client to the ScenarioRun.
// The Client is a clean unmodified client that has no mocks.
// The client should be suitable to perform operations against k8s with nothing getting in a way.
type ScenarioClientFn[K client.Object] func(*ScenarioRun[K])

// ScenarioHydrationFn is a signature of a function that hydrates resources for this ScenarioRun.
// It stores main resource as a Container on the run, but there could be additional resources created in the cluster too.
// The checks in this scenario should be able to find these additional resources on their own.
type ScenarioHydrationFn[K client.Object] func(*ScenarioRun[K])

// ScenarioFakeClientFn is a signature of a function that adds a fake client to the ScenarioRun.
// This client may be modified to mock certain calls in order to reproduce this scenario.
// It is stored as FakeClient on the run.
type ScenarioFakeClientFn[K client.Object] func(*ScenarioRun[K])

// ScenarioReconcilerFn is a signature of a function that returns a reconciler.
type ScenarioReconcilerFn[R reconcile.TypedReconciler[reconcile.Request]] func(client.Client) R

// ScenarioCheckFn is a signature of a function that checks the results of this ScenarioRun.
type ScenarioCheckFn[K client.Object] func(*ScenarioRun[K])

// ScenarioRegisterCallback is a signature of a callback function to register this scenario to the collector.
type ScenarioRegisterCallback[K client.Object] func(*ScenarioWithCheck[K])

// ScenarioItFn is a signature of a function that is called from ginkgo to run a scenario.
type ScenarioItFn func(context.Context, *Client, *ObjectContainer[*corev1.Namespace])

// ScenarioCollector is a collector to instruct ginkgo.
// The idea is that the collector is instantiated once per the ginkgo context.
// Scenarios are getting registered to the collector during configuration phase using ScenarioRegisterCallback.
// In the runtime phase, registered scenarios are executed using their ScenarioItFn functions.
type ScenarioCollector[K client.Object, R reconcile.TypedReconciler[reconcile.Request]] struct {
	reconcilerFn ScenarioReconcilerFn[R]
	checks       []*ScenarioWithCheck[K]
}

// ScenarioTemplate is a template for a scenario.
// The seedFn must return a resource initialized with a name.
// Namespace will be added to it automatically during the execution.
// Seed resource might return an object with some additional data, and it will be preserved too.
// However, sometimes it be hard to prepare complex resource with dependencies without access to the Client or the ScenarioRun.
// Hydration functions can be used for that instead.
type ScenarioTemplate[K client.Object] struct {
	templateName string
	seedFn       ScenarioSeedFn[K]
}

// ScenarioWithHydration is a scenario instantiated from a template with a set of hydration functions.
// Hydration functions are used to prepare resources for the test, and create them in the cluster.
// Hydration functions are expected to store main resource as the Container on this ScenarioRun.
// Separation of base hydration function and patches allow to provide a reusable baseline for scenario reusability.
// Patches can be discarded separately from the base to be able to reuse the base with different set of patches to reproduce totally different scenario.
type ScenarioWithHydration[K client.Object] struct {
	*ScenarioTemplate[K]
	hydrationFn        ScenarioHydrationFn[K]
	hydrationPatchesFn []ScenarioHydrationFn[K]
}

// ScenarioWithFakeClient is a hydrated scenario with functions that can prepare a fake client.
// Fake client is used during the scenario to simulate certain conditions.
// It may or may not have mocks registered on it.
// Functions must store the client to the ScenarioRun as FakeClient.
// Separation of base fake client function and patches allow to provide a reusable baseline for scenario reusability.
// Patches can be discarded separately from the base to be able to reuse the base with different set of patches to reproduce totally different scenario.
type ScenarioWithFakeClient[K client.Object] struct {
	*ScenarioWithHydration[K]
	fakeClientFn        ScenarioFakeClientFn[K]
	fakeClientPatchesFn []ScenarioFakeClientFn[K]
}

// ScenarioWithCheck is hydrated and has a fake client and check functions describe how to validate result upon execution.
type ScenarioWithCheck[K client.Object] struct {
	*ScenarioWithFakeClient[K]
	checkName string
	checkFn   []ScenarioCheckFn[K]
}

// ScenarioRun is a state of the scenario during the execution.
// Various functions use pointer to the run to communicate with each other.
type ScenarioRun[K client.Object] struct {
	// Context is a context for the scenario.
	Context context.Context

	// Client is a clean client that can be used to perform operations against k8s.
	// This must be a clean client with no mocks.
	Client *Client

	// Namespace is a namespace object that is used to create resources in the cluster.
	Namespace *ObjectContainer[*corev1.Namespace]

	// FakeClient is a fake client that can be used to mock certain calls.
	FakeClient *FakeClient

	// SeedObject is a resource that was seeded by the seed function.
	SeedObject K

	// Container is a container that holds the main resource created during hydration.
	Container *ObjectContainer[K]

	// ReconcileResult is a result of the reconciliation.
	ReconcileResult reconcile.Result

	// ReconcileError is an error that occurred during the reconciliation.
	ReconcileError error
}

// ScenarioIt is a wrapper for ginkgo.
// It represents executable unit of code for the It function in ginkgo.
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

// Hydrate hydrates the scenario with a base hydration function.
func (s *ScenarioTemplate[K]) Hydrate(
	hydrationFn ScenarioHydrationFn[K],
) *ScenarioWithHydration[K] {
	return &ScenarioWithHydration[K]{
		ScenarioTemplate:   s,
		hydrationFn:        hydrationFn,
		hydrationPatchesFn: []ScenarioHydrationFn[K]{},
	}
}

// HydrateWithContainer is a shortcut hydration function that instantiates the seed object as a container.
// Container will NOT be created in the cluster.
// Useful if the project to be customized with hydration patches before creation,
// or if the object must not exist for this check.
func (s *ScenarioTemplate[K]) HydrateWithContainer() *ScenarioWithHydration[K] {
	fn := func(run *ScenarioRun[K]) {
		Expect(run).NotTo(BeNil())
		Expect(run.SeedObject).NotTo(BeNil())
		Expect(run.SeedObject.GetName()).NotTo(BeEmpty())
		Expect(run.SeedObject.GetNamespace()).ToNot(BeEmpty())
		container := NewObjectContainer(run.SeedObject)
		run.Container = container
	}
	return s.Hydrate(fn)
}

// HydrateWithClientUsingContainer is a shortcut hydration function that creates container in the cluster using clean client.
// Container must already exist on the run.
func (s *ScenarioTemplate[K]) HydrateWithClientUsingContainer() *ScenarioWithHydration[K] {
	fn := func(run *ScenarioRun[K]) {
		Expect(run).NotTo(BeNil())
		Expect(run.Client).NotTo(BeNil())
		Expect(run.Container).NotTo(BeNil())
		Expect(run.Container.ClientObject()).NotTo(BeNil())
		Expect(run.Container.Object().GetName()).NotTo(BeEmpty())
		Expect(run.Container.Object().GetNamespace()).NotTo(BeNil())
		Expect(
			run.Client.CreateContainer(run.Context, run.Container.ClientObject()),
		).To(Succeed())
		Expect(
			run.Client.GetContainer(run.Context, run.Container.ClientObject()),
		).To(Succeed())
	}
	return s.Hydrate(fn)
}

// HydrateWithClientCreatingContainer is a shortcut hydration function that runs HydrateWithClientUsingContainer after HydrateWithContainer.
func (s *ScenarioTemplate[K]) HydrateWithClientCreatingContainer() *ScenarioWithHydration[K] {
	containerFn := s.HydrateWithContainer().hydrationFn
	clientFn := s.HydrateWithClientUsingContainer().hydrationFn
	n := s.Hydrate(func(run *ScenarioRun[K]) {})
	n.hydrationFn = func(run *ScenarioRun[K]) {
		containerFn(run)
		clientFn(run)
	}
	return n
}

// Hydrate adds hydration patch to this scenario.
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

// WithFakeClient adds a fake client base function to the scenario.
// If fakeClientFn is nil - will use a clean client as fake client in the scenario.
// It means that no mocks are necessary for this scenario.
func (s *ScenarioWithHydration[K]) WithFakeClient(
	fakeClientFn ScenarioFakeClientFn[K],
) *ScenarioWithFakeClient[K] {
	if fakeClientFn == nil {
		fakeClientFn = func(run *ScenarioRun[K]) {
			Expect(run).NotTo(BeNil())
			Expect(run.Client).NotTo(BeNil())
			run.FakeClient = &FakeClient{Client: run.Client}
		}
	}
	return &ScenarioWithFakeClient[K]{
		ScenarioWithHydration: s,
		fakeClientFn:          fakeClientFn,
	}
}

// WithFakeClient adds fake client patch functions to the scenario.
func (s *ScenarioWithFakeClient[K]) WithFakeClient(
	fakeClientFn ...ScenarioFakeClientFn[K],
) *ScenarioWithFakeClient[K] {
	s.fakeClientPatchesFn = append(s.fakeClientPatchesFn, fakeClientFn...)
	return s
}

// WithFakeClientThatFailsToGetResource adds additional mock to simulate failure to get the resource.
func (s *ScenarioWithFakeClient[K]) WithFakeClientThatFailsToGetResource() *ScenarioWithFakeClient[K] {
	fn := func(run *ScenarioRun[K]) {
		run.FakeClient.
			WithGetFunction(run.Container.ClientObject(),
				func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return errors.NewBadRequest("fake error getting resource")
				},
			)
	}
	return s.WithFakeClient(fn)
}

// WithFakeClientThatFailsToUpdateStatus adds additional mock to simulate failure to update the status.
func (s *ScenarioWithFakeClient[K]) WithFakeClientThatFailsToUpdateStatus() *ScenarioWithFakeClient[K] {
	fn := func(run *ScenarioRun[K]) {
		Expect(run).NotTo(BeNil())
		Expect(run.FakeClient).NotTo(BeNil())
		run.FakeClient.
			WithStatusUpdateFunction(run.Container.ClientObject(),
				func(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
					return errors.NewBadRequest("fake error updating status")
				},
			)
	}
	return s.WithFakeClient(fn)
}

// RemoveClient returns HydratedScenario ready to be instantiated with another client.
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

// Branch allows to branch out a scenario as a copy for additional checks without the intention to extend it.
// Within the branch - scenario can continue like normally.
// The function returns the original scenario, so it can be chained further down by "forgetting" the branch timeline.
func (s *ScenarioWithFakeClient[K]) Branch(branch func(*ScenarioWithFakeClient[K])) *ScenarioWithFakeClient[K] {
	clone := &ScenarioWithFakeClient[K]{
		ScenarioWithHydration: s.RemoveClient(),
		fakeClientFn:          s.fakeClientFn,
		fakeClientPatchesFn:   make([]ScenarioFakeClientFn[K], len(s.fakeClientPatchesFn)),
	}
	copy(clone.fakeClientPatchesFn, s.fakeClientPatchesFn)
	branch(clone)
	return s
}

// BranchResourceNotFoundCheck is a shortcut function that uses scenario client and a set of checks that are expecting clean exit with no re-queue.
// Acts as a separate branch, does not modify the state, and allows to continue the scenario without impacting it.
func (s *ScenarioWithFakeClient[K]) BranchResourceNotFoundCheck(callback ScenarioRegisterCallback[K]) *ScenarioWithFakeClient[K] {
	clone := &ScenarioWithFakeClient[K]{
		ScenarioWithHydration: s.RemoveClient(),
		fakeClientFn:          s.fakeClientFn,
		fakeClientPatchesFn:   make([]ScenarioFakeClientFn[K], len(s.fakeClientPatchesFn)),
	}
	copy(clone.fakeClientPatchesFn, s.fakeClientPatchesFn)
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

// BranchFailureToGetResourceCheck is a shortcut function that uses a WithClientFromParentThatFailsToGetResource() and a set of checks that are expecting a corresponding error from it.
// Acts as a separate branch, does not modify the state, and allows to continue the scenario without impacting it.
func (s *ScenarioWithFakeClient[K]) BranchFailureToGetResourceCheck(callback ScenarioRegisterCallback[K]) *ScenarioWithFakeClient[K] {
	clone := &ScenarioWithFakeClient[K]{
		ScenarioWithHydration: s.RemoveClient(),
		fakeClientFn:          s.fakeClientFn,
		fakeClientPatchesFn:   make([]ScenarioFakeClientFn[K], len(s.fakeClientPatchesFn)),
	}
	copy(clone.fakeClientPatchesFn, s.fakeClientPatchesFn)
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

// BranchFailureToUpdateStatusCheck is a shortcut function that uses a WithClientFromParentThatFailsToUpdateStatus() and a set of check that are expecting a corresponding error from it.
// Acts as a separate branch, does not modify the state, and allows to continue the scenario without impacting it.
func (s *ScenarioWithFakeClient[K]) BranchFailureToUpdateStatusCheck(callback ScenarioRegisterCallback[K]) *ScenarioWithFakeClient[K] {
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

// WithCheck adds a check to this scenario.
func (s *ScenarioWithCheck[K]) WithCheck(
	checkFn ScenarioCheckFn[K],
) *ScenarioWithCheck[K] {
	s.checkFn = append(s.checkFn, checkFn)
	return s
}

// Commit registers a scenario in the collector by calling a callback.
// Returns a ScenarioWithFakeClient ready for another set of checks with additional mocks.
func (s *ScenarioWithCheck[K]) Commit(callback ScenarioRegisterCallback[K]) *ScenarioWithFakeClient[K] {
	callback(s)
	clone := &ScenarioWithFakeClient[K]{
		ScenarioWithHydration: s.RemoveClient(),
		fakeClientFn:          s.fakeClientFn,
		fakeClientPatchesFn:   make([]ScenarioFakeClientFn[K], len(s.fakeClientPatchesFn)),
	}
	copy(clone.fakeClientPatchesFn, s.fakeClientPatchesFn)
	return clone
}

// Collect is a callback function to register a scenario in the collector.
func (c *ScenarioCollector[K, R]) Collect(check *ScenarioWithCheck[K]) {
	c.checks = append(c.checks, check)
}

// All is a function that returns all scenarios that was registered as a list of ScenarioIt.
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
