package autoscaler

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func findByRef[to any](
	ctx context.Context,
	scheme *runtime.Scheme,
	restMapper meta.RESTMapper,
	k8sClient client.Client,
	namespace string,
	ref corev1.TypedLocalObjectReference,
) (to, error) {

	log := log.FromContext(ctx)

	apiGroup := ""
	if ref.APIGroup != nil {
		apiGroup = *ref.APIGroup
	}

	gk := schema.GroupKind{
		Group: apiGroup,
		Kind:  ref.Kind,
	}

	mapping, err := restMapper.RESTMapping(gk, "")
	if err != nil {
		log.Error(err, "Failed to map GVK to REST mapping")
		return *new(to), err
	}

	gvk := schema.GroupVersionKind{
		Group:   apiGroup,
		Version: mapping.GroupVersionKind.Version,
		Kind:    ref.Kind,
	}

	obj, err := scheme.New(gvk)
	if err != nil {
		return *new(to), fmt.Errorf("failed to create object for GVK %s: %w", gvk.String(), err)
	}

	clientObj, ok := obj.(client.Object)
	if !ok {
		return *new(to), fmt.Errorf("object for GVK %s does not implement client.Object", gvk.String())
	}

	key := types.NamespacedName{Name: ref.Name, Namespace: namespace}
	if err := k8sClient.Get(ctx, key, clientObj); err != nil {
		return *new(to), fmt.Errorf("failed to fetch resource %s: %w", key, err)
	}

	target, ok := any(clientObj).(to)
	if !ok {
		log.Error(nil, "Resource does not implement the required type", "Type", reflect.TypeOf((*to)(nil)).Elem())
		return *new(to), fmt.Errorf("resource %s does not implement required type %T", key, new(to))
	}

	return target, nil
}
