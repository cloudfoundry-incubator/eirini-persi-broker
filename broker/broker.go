package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pivotal-cf/brokerapi"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"code.cloudfoundry.org/eirini-persi-broker/config"
)

// KubeVolumeBroker is a broker for Kubernetes Volumes
type KubeVolumeBroker struct {
	KubeClient kubernetes.Interface
	Config     config.Config
	Context context.Context
}

// userMountConfiguration represents the configuration the
// user can pass when doing cf bind ...
type userMountConfiguration struct {
	Directory string `json:"dir"`
}

// userConfiguration represents the configuration the
// user can pass when doing cf create-service ...
type userConfiguration struct {
	Size string `json:"size"`
	AccessMode string `json:"access_mode"`
}

// Services returns a list with one item, the service for provisioning kubernetes volumes
func (b *KubeVolumeBroker) Services(ctx context.Context) ([]brokerapi.Service, error) {
	planList := make([]brokerapi.ServicePlan, len(b.Config.ServiceConfiguration.Plans))

	for idx, plan := range b.Config.ServiceConfiguration.Plans {
		planList[idx] = brokerapi.ServicePlan{
			Name:        plan.Name,
			Description: plan.Description,
			Free:        &plan.Free,
			ID:          plan.ID,
		}
	}

	return []brokerapi.Service{
		brokerapi.Service{
			ID:          b.Config.ServiceConfiguration.ServiceID,
			Name:        b.Config.ServiceConfiguration.ServiceName,
			Description: b.Config.ServiceConfiguration.Description,
			Bindable:    true,
			Plans:       planList,

			Metadata: &brokerapi.ServiceMetadata{
				DisplayName:         b.Config.ServiceConfiguration.DisplayName,
				LongDescription:     b.Config.ServiceConfiguration.LongDescription,
				DocumentationUrl:    b.Config.ServiceConfiguration.DocumentationURL,
				SupportUrl:          b.Config.ServiceConfiguration.SupportURL,
				ImageUrl:            fmt.Sprintf("data:image/png;base64,%s", b.Config.ServiceConfiguration.IconImage),
				ProviderDisplayName: b.Config.ServiceConfiguration.ProviderDisplayName,
			},
			Tags: []string{
				"eirini",
				"kubernetes",
				"storage",
			},
			Requires: []brokerapi.RequiredPermission{
				brokerapi.PermissionVolumeMount,
			},
		},
	}, nil
}

// Provision creates a Kubernetes PVC
func (b *KubeVolumeBroker) Provision(ctx context.Context, instanceID string, serviceDetails brokerapi.ProvisionDetails, asyncAllowed bool) (spec brokerapi.ProvisionedServiceSpec, err error) {
	spec = brokerapi.ProvisionedServiceSpec{}

	// Resolve the plan for this service instance
	if serviceDetails.PlanID == "" {
		return spec, errors.New("plan_id required")
	}

	var plan *config.Plan
	for _, p := range b.Config.ServiceConfiguration.Plans {
		if p.ID == serviceDetails.PlanID {
			plan = &p
			break
		}
	}

	if plan == nil {
		return spec, errors.New("plan_id not recognized")
	}

	// See if the instance already exists
	volumeExists, _, err := b.instanceExists(instanceID)
	if err != nil {
		return spec, errors.Wrap(err, "error provisioning")
	}

	// If the persistent volume claim already exists, return a specific error
	if volumeExists {
		return spec, brokerapi.ErrInstanceAlreadyExists
	}

	// Figure out how much storage to provision
	var userConfig userConfiguration
	if len(serviceDetails.RawParameters) > 0 {
		err = json.Unmarshal(serviceDetails.RawParameters, &userConfig)
		if err != nil {
			return spec, errors.Wrap(err, "error unmarshaling json user configuration")
		}
	}
	size := userConfig.Size
	if size == "" {
		size = plan.DefaultSize
	}
	if size == "" {
		return spec, errors.New("plan doesn't have a default size")
	}

	accessMode := userConfig.AccessMode
	if accessMode == "" {
		accessMode = plan.DefaultAccessMode
	}
	if accessMode == "" {
		accessMode = "ReadWriteMany"
	}

	quantity, err := resource.ParseQuantity(size)
	if err != nil {
		return spec, errors.Wrap(err, "invalid quantity string")
	}

	_, err = b.KubeClient.CoreV1().PersistentVolumeClaims(b.Config.Namespace).Create(b.Context, &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: instanceID,
			Labels: map[string]string{
				"service-id":      serviceDetails.ServiceID,
				"plan-id":         serviceDetails.PlanID,
				"organization-id": serviceDetails.OrganizationGUID,
				"space-id":        serviceDetails.SpaceGUID,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: plan.StorageClass,
			AccessModes: []corev1.PersistentVolumeAccessMode{
				accessMode,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": quantity,
				},
			},
		},
	},metav1.CreateOptions{})

	if err != nil {
		return spec, errors.Wrap(err, "error provisioning")
	}

	spec.IsAsync = false
	// TODO: point to a Kubernetes Dashboard URL, if configured
	spec.DashboardURL = ""

	return spec, nil
}

// Deprovision deletes a Kubernetes PVC
func (b *KubeVolumeBroker) Deprovision(ctx context.Context, instanceID string, details brokerapi.DeprovisionDetails, asyncAllowed bool) (brokerapi.DeprovisionServiceSpec, error) {
	spec := brokerapi.DeprovisionServiceSpec{}

	volumeExists, _, err := b.instanceExists(instanceID)
	if err != nil {
		return spec, errors.Wrap(err, "error deprovisioning")
	}

	// If the volume doesn't exist, the service instance doesn't exist
	if !volumeExists {
		return spec, brokerapi.ErrInstanceDoesNotExist
	}

	// Delete the PVC
	err = b.KubeClient.CoreV1().PersistentVolumeClaims(b.Config.Namespace).Delete(b.Context,instanceID, metav1.DeleteOptions{})
	if err != nil {
		return spec, errors.Wrap(err, "error deleting persistent volume claim for deprovisioning")
	}

	return spec, nil
}

// Bind adds an annotation to the service instance PVC
func (b *KubeVolumeBroker) Bind(ctx context.Context, instanceID, bindingID string, details brokerapi.BindDetails, asyncAllowed bool) (brokerapi.Binding, error) {
	spec := brokerapi.Binding{}

	volumeExists, pvc, err := b.instanceExists(instanceID)
	if err != nil {
		return spec, errors.Wrap(err, "error binding")
	}

	// If the volume doesn't exist, the service instance doesn't exist
	if !volumeExists {
		return spec, brokerapi.ErrInstanceDoesNotExist
	}

	// If the annotation already exists on the PVC, we return a specific error
	if _, ok := pvc.Annotations[bindingIDAnnotation(bindingID)]; ok {
		return spec, brokerapi.ErrBindingAlreadyExists
	}

	// Resolve the mount directory
	var userMount userMountConfiguration
	if len(details.RawParameters) > 0 {
		err = json.Unmarshal(details.RawParameters, &userMount)
		if err != nil {
			return spec, errors.Wrap(err, "error unmarshaling json user configuration")
		}
	}
	containerDir := userMount.Directory
	if containerDir == "" {
		containerDir = fmt.Sprintf("/var/vcap/data/%s", bindingID)
	}

	// Add the annotation
	if pvc.Annotations == nil {
		pvc.Annotations = map[string]string{}
	}
	pvc.Annotations[bindingIDAnnotation(bindingID)] = containerDir
	_, err = b.KubeClient.CoreV1().PersistentVolumeClaims(b.Config.Namespace).Update(b.Context,pvc,metav1.UpdateOptions{})
	if err != nil {
		return spec, errors.Wrap(err, "error updating persistent volume claim annotations for binding")
	}

	// If there's no storage class on the pvc, something's wrong
	if pvc.Spec.StorageClassName == nil {
		return spec, errors.New("pvc has a nil storage class")
	}

	storageClassName := *pvc.Spec.StorageClassName
	spec.Credentials = map[string]interface{}{
		"volume_id": pvc.Name,
	}
	spec.VolumeMounts = []brokerapi.VolumeMount{
		{
			Driver:       storageClassName,
			ContainerDir: containerDir,
			Mode:         "rw",
			DeviceType:   "shared",
			Device: brokerapi.SharedDevice{
				VolumeId: pvc.Name,
			},
		},
	}
	return spec, nil
}

// Unbind removes the binding annotation from the appropriate PVC
func (b *KubeVolumeBroker) Unbind(ctx context.Context, instanceID, bindingID string, details brokerapi.UnbindDetails, asyncAllowed bool) (brokerapi.UnbindSpec, error) {
	spec := brokerapi.UnbindSpec{}

	volumeExists, pvc, err := b.instanceExists(instanceID)
	if err != nil {
		return spec, errors.Wrap(err, "error unbinding")
	}

	// If the volume doesn't exist, the service instance doesn't exist
	if !volumeExists {
		return spec, brokerapi.ErrInstanceDoesNotExist
	}

	// If the annotation doesn't exist on the PVC, we return a specific error
	if len(pvc.Annotations) == 0 {
		return spec, brokerapi.ErrBindingDoesNotExist
	}

	if _, ok := pvc.Annotations[bindingIDAnnotation(bindingID)]; !ok {
		return spec, brokerapi.ErrBindingDoesNotExist
	}

	// Remove the annotation
	delete(pvc.Annotations, bindingIDAnnotation(bindingID))
	_, err = b.KubeClient.CoreV1().PersistentVolumeClaims(b.Config.Namespace).Update(b.Context,pvc,metav1.UpdateOptions{})
	if err != nil {
		return spec, errors.Wrap(err, "error updating persistent volume claim annotations for unbinding")
	}

	return spec, nil
}

// GetInstance finds the correct PVC and reconstructs an instance spec
func (b *KubeVolumeBroker) GetInstance(ctx context.Context, instanceID string) (brokerapi.GetInstanceDetailsSpec, error) {
	spec := brokerapi.GetInstanceDetailsSpec{}

	volumeExists, pvc, err := b.instanceExists(instanceID)
	if err != nil {
		return spec, errors.Wrap(err, "error getting instance")
	}

	// If the volume doesn't exist, the service instance doesn't exist
	if !volumeExists {
		return spec, brokerapi.ErrInstanceDoesNotExist
	}

	var ok bool

	// TODO: set dashboard URL
	if spec.PlanID, ok = pvc.Labels["plan-id"]; !ok {
		return spec, errors.New("plan-id label missing from pvc")
	}
	if spec.ServiceID, ok = pvc.Labels["service-id"]; !ok {
		return spec, errors.New("service-id label missing from pvc")
	}

	return spec, nil
}

// GetBinding finds the correct PVC and its binding annotation and reconstructs a binding spec
func (b *KubeVolumeBroker) GetBinding(ctx context.Context, instanceID, bindingID string) (brokerapi.GetBindingSpec, error) {
	spec := brokerapi.GetBindingSpec{}

	volumeExists, pvc, err := b.instanceExists(instanceID)
	if err != nil {
		return spec, errors.Wrap(err, "error getting binding")
	}

	// If the volume doesn't exist, the service instance doesn't exist
	if !volumeExists {
		return spec, brokerapi.ErrInstanceDoesNotExist
	}

	// If the annotation doesn't exist on the PVC, we return a specific error
	if len(pvc.Annotations) == 0 {
		return spec, brokerapi.ErrBindingDoesNotExist
	}

	containerDir, ok := pvc.Annotations[bindingIDAnnotation(bindingID)]
	if !ok {
		return spec, brokerapi.ErrBindingDoesNotExist
	}

	// If there's no storage class on the pvc, something's wrong
	if pvc.Spec.StorageClassName == nil {
		return spec, errors.New("pvc has a nil storage class")
	}
	storageClassName := *pvc.Spec.StorageClassName

	spec.VolumeMounts = []brokerapi.VolumeMount{
		{
			Driver:       storageClassName,
			ContainerDir: containerDir,
			Mode:         "rw",
			DeviceType:   "shared",
			Device: brokerapi.SharedDevice{
				VolumeId: pvc.Name,
			},
		},
	}

	return spec, nil
}

// LastBindingOperation is currently a noop
func (b *KubeVolumeBroker) LastBindingOperation(ctx context.Context, instanceID, bindingID string, details brokerapi.PollDetails) (brokerapi.LastOperation, error) {
	return brokerapi.LastOperation{}, nil
}

// LastOperation is currently a noop
func (b *KubeVolumeBroker) LastOperation(ctx context.Context, instanceID string, details brokerapi.PollDetails) (brokerapi.LastOperation, error) {
	return brokerapi.LastOperation{}, nil
}

// Update is currently a noop
func (b *KubeVolumeBroker) Update(ctx context.Context, instanceID string, details brokerapi.UpdateDetails, asyncAllowed bool) (brokerapi.UpdateServiceSpec, error) {
	return brokerapi.UpdateServiceSpec{}, nil
}

func (b *KubeVolumeBroker) instanceExists(instanceID string) (bool, *corev1.PersistentVolumeClaim, error) {
	pvc, err := b.KubeClient.CoreV1().PersistentVolumeClaims(b.Config.Namespace).Get(b.Context,instanceID, metav1.GetOptions{})

	if apierrors.IsNotFound(err) {
		return false, nil, nil
	}

	if err != nil {
		return false, nil, errors.Wrap(err, "error listing persistent volumes")
	}

	return true, pvc, nil
}

func bindingIDAnnotation(bindingID string) string {
	return "eirini-broker-binding-" + bindingID
}

func isBindingIDAnnotation(annotationKey string) bool {
	return strings.HasPrefix(annotationKey, "eirini-broker-binding-")
}
