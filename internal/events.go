package internal

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	"github.com/rexagod/crsm/internal/version"
	"github.com/rexagod/crsm/pkg/apis/crsm/v1alpha1"
	clientset "github.com/rexagod/crsm/pkg/generated/clientset/versioned"
)

// eventType represents the type of event received from the informer.
type eventType int

const (
	addEvent eventType = iota
	updateEvent
	deleteEvent
)

func (e eventType) String() string {
	return []string{"addEvent", "updateEvent", "deleteEvent"}[e]
}

// crsmHandler knows how to handle CRSM events.
type crsmHandler struct {

	// kubeClientset is the clientset used to interact with the Kubernetes API.
	kubeClientset kubernetes.Interface

	// crsmClientset is the clientset used to update the status of the CRSM resource.
	crsmClientset clientset.Interface

	// dynamicClientset is the dynamic clientset used to build stores for different objects.
	dynamicClientset dynamic.Interface
}

// newCRSMHandler creates a new crsmHandler.
func newCRSMHandler(kubeClientset kubernetes.Interface, crsmClientset clientset.Interface, dynamicClientset dynamic.Interface) *crsmHandler {
	return &crsmHandler{
		kubeClientset:    kubeClientset,
		crsmClientset:    crsmClientset,
		dynamicClientset: dynamicClientset,
	}
}

// HandleEvent handles events received from the informer.
func (h *crsmHandler) handleEvent(
	ctx context.Context,
	crsmUIDToStoresMap map[types.UID][]*StoreType,
	event string,
	o metav1.Object,
	tryNoCache bool,
) error {
	logger := klog.FromContext(ctx)

	// Resolve the object type.
	resource, ok := o.(*v1alpha1.CustomResourceStateMetricsResource)
	if !ok {
		logger.Error(fmt.Errorf("failed to cast object to %s", resource.GetObjectKind()), "cannot handle event")
		return nil // Do not requeue.
	}
	kObj := klog.KObj(resource).String()

	// Preemptively update the resource metadata. We poll here to avoid same resource versions across update bursts.
	err := h.updateMetadata(ctx, resource)
	if err != nil {
		logger.Error(fmt.Errorf("failed to update metadata for %s: %w", kObj, err), "cannot handle event")
		return nil // Do not requeue.
	}

	// Update resource status.
	resource, err = h.emitSuccessOnResource(ctx, resource, metav1.ConditionFalse, fmt.Sprintf("Event handler received event: %s", event))
	if err != nil {
		logger.Error(fmt.Errorf("failed to emit success on %s: %w", kObj, err), "cannot update the resource")
		return nil // Do not requeue.
	}

	// Process the fetched configuration.
	configurationYAML := resource.Spec.ConfigurationYAML
	if configurationYAML == "" {

		// This should never happen owing to the Kubebuilder check in place.
		logger.Error(stderrors.New("configuration YAML is empty"), "cannot process the resource")
		h.emitFailureOnResource(ctx, resource, "Configuration YAML is empty")
		return nil
	}
	configurerInstance := newConfigurer(ctx, h.dynamicClientset, resource)

	// dropStores drops associated stores between resource changes.
	dropStores := func() {
		resourceUID := resource.GetUID()
		if _, ok = crsmUIDToStoresMap[resourceUID]; ok {

			// The associated stores are only reachable through the map. Deleting them will trigger the GC.
			delete(crsmUIDToStoresMap, resourceUID)
		}
	}

	// Handle the event.
	switch event {

	// Build all associated stores.
	case addEvent.String(), updateEvent.String():
		dropStores()
		err = configurerInstance.parse(configurationYAML)
		if err != nil {
			logger.Error(fmt.Errorf("failed to parse configuration YAML: %w", err), "cannot process the resource")
			h.emitFailureOnResource(ctx, resource, fmt.Sprintf("Failed to parse configuration YAML: %s", err))
			return nil
		}
		configurerInstance.build(crsmUIDToStoresMap, tryNoCache)

	// Drop all associated stores.
	case deleteEvent.String():
		dropStores()

	// This should never happen.
	default:
		logger.Error(fmt.Errorf("unknown event type (%s)", event), "cannot process the resource")
		h.emitFailureOnResource(ctx, resource, fmt.Sprintf("Unknown event type: %s", event))
		return nil
	}

	// Update the status of the resource.
	_, err = h.emitSuccessOnResource(ctx, resource, metav1.ConditionTrue, fmt.Sprintf("Event handler successfully processed event: %s", event))
	if err != nil {
		logger.Error(fmt.Errorf("failed to emit success on %s: %w", kObj, err), "cannot update the resource")
		return nil // Do not requeue.
	}

	return nil
}

// emitSuccessOnResource emits a success condition on the given resource.
func (h *crsmHandler) emitSuccessOnResource(
	ctx context.Context,
	gotResource *v1alpha1.CustomResourceStateMetricsResource,
	conditionBool metav1.ConditionStatus,
	message string,
) (*v1alpha1.CustomResourceStateMetricsResource, error) {
	kObj := klog.KObj(gotResource).String()

	resource, err := h.crsmClientset.CrsmV1alpha1().CustomResourceStateMetricsResources(gotResource.GetNamespace()).
		Get(ctx, gotResource.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get %s: %w", kObj, err)
	}
	resource.Status.Set(resource, metav1.Condition{
		Type:    v1alpha1.ConditionType[v1alpha1.ConditionTypeProcessed],
		Status:  conditionBool,
		Message: message,
	})
	resource, err = h.crsmClientset.CrsmV1alpha1().CustomResourceStateMetricsResources(resource.GetNamespace()).
		UpdateStatus(ctx, resource, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update the status of %s: %w", kObj, err)
	}

	return resource, nil
}

// emitFailureOnResource emits a failure condition on the given resource.
func (h *crsmHandler) emitFailureOnResource(
	ctx context.Context,
	gotResource *v1alpha1.CustomResourceStateMetricsResource,
	message string,
) /* Don't return the most recent resource since this call should always precede an empty return. */ {
	kObj := klog.KObj(gotResource).String()

	resource, err := h.crsmClientset.CrsmV1alpha1().CustomResourceStateMetricsResources(gotResource.GetNamespace()).
		Get(ctx, gotResource.GetName(), metav1.GetOptions{})
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to get %s: %w", kObj, err))
		return
	}
	resource.Status.Set(resource, metav1.Condition{
		Type:    v1alpha1.ConditionType[v1alpha1.ConditionTypeFailed],
		Status:  metav1.ConditionTrue,
		Message: message,
	})
	_, err = h.crsmClientset.CrsmV1alpha1().CustomResourceStateMetricsResources(resource.GetNamespace()).
		UpdateStatus(ctx, resource, metav1.UpdateOptions{})
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to emit failure on %s: %w", kObj, err))
		return
	}
}

// updateMetadata updates the metadata of the CRSMR resource.
func (h *crsmHandler) updateMetadata(ctx context.Context, resource *v1alpha1.CustomResourceStateMetricsResource) error {
	logger := klog.FromContext(ctx)
	kObj := klog.KObj(resource).String()

	err := wait.PollUntilContextTimeout(ctx, time.Second, time.Minute, false, func(context.Context) (
		bool,
		error,
	) {
		gotResource, err := h.crsmClientset.CrsmV1alpha1().CustomResourceStateMetricsResources(resource.GetNamespace()).
			Get(ctx, resource.GetName(), metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to get %s: %w", kObj, err)
		}
		resource = gotResource.DeepCopy() // Ensure we are working with the latest resourceVersion.

		// Add relevant metadata information to the resource.
		// Build relevant labels.
		if resource.Labels == nil {
			resource.Labels = make(map[string]string)
		}
		controllerNameSanitized := strings.ReplaceAll(version.ControllerName, "_", "-")
		resource.Labels["app.kubernetes.io/managed-by"] = controllerNameSanitized
		revisionSHA := regexp.MustCompile(`revision:\s*(\S+)\)`).FindStringSubmatch(version.Version())
		if len(revisionSHA) > 1 {
			resource.Labels["app.kubernetes.io/version"] = revisionSHA[1]
		} else {
			logger.Error(stderrors.New("failed to get revision SHA, continuing anyway"), "cannot set version label")
		}

		// Set up CR GC.
		namespace, found := os.LookupEnv("POD_NAMESPACE")
		if found {
			ownerRef, err2 := h.kubeClientset.AppsV1().Deployments(namespace).Get(ctx, controllerNameSanitized, metav1.GetOptions{})
			if err2 != nil {
				return false, fmt.Errorf("failed to get owner reference: %w", err)
			}
			resource.SetOwnerReferences([]metav1.OwnerReference{
				{
					// Use apps/v1 API for the Deployment GVK since it's not populated.
					APIVersion:         appsv1.SchemeGroupVersion.String(),
					Kind:               "Deployment",
					Name:               ownerRef.GetName(),
					UID:                ownerRef.GetUID(),
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(false), // Allow removing the CR without removing the controller.
				},
			})
		} else {
			logger.Error(stderrors.New("failed to get namespace, continuing anyway"), "cannot set ownerReference")
		}

		// Compare resource with the fetched resource.
		resource, err = h.crsmClientset.CrsmV1alpha1().CustomResourceStateMetricsResources(resource.GetNamespace()).
			Update(ctx, resource, metav1.UpdateOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to update %s: %w", kObj, err)
		}

		return true, nil
	})

	if err != nil {
		return fmt.Errorf("failed while polling for %s: %w", kObj, err)
	}

	return nil
}
