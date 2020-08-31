package broker_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"code.cloudfoundry.org/eirini-persi-broker/broker"
	brokerconfig "code.cloudfoundry.org/eirini-persi-broker/config"
)

var _ = Describe("broker", func() {
	var (
		testBroker broker.KubeVolumeBroker
		kubeClient kubernetes.Interface
	)

	BeforeEach(func() {
		kubeClient = fake.NewSimpleClientset()
		config := brokerconfig.Config{
			ServiceConfiguration: DefaultServiceConfiguration(),
			Namespace:            DefaultNamespace,
		}

		testBroker = broker.KubeVolumeBroker{
			KubeClient: kubeClient,
			Config:     config,
		}
	})

	Describe("service", func() {
		Context("when using the marketplace", func() {
			It("returns one configured service", func() {
				services, err := testBroker.Services(context.Background())

				Expect(err).NotTo(HaveOccurred())
				Expect(len(services)).To(Equal(1))
				Expect(services[0].ID).To(Equal(DefaultServiceID))
				Expect(services[0].Name).To(Equal(DefaultServiceName))
				Expect(services[0].Bindable).To(Equal(true))
				Expect(services[0].Tags).To(Equal([]string{"eirini", "kubernetes", "storage"}))
				Expect(services[0].Requires).To(Equal([]brokerapi.RequiredPermission{brokerapi.PermissionVolumeMount}))
			})

			It("returns the configured plans", func() {
				services, err := testBroker.Services(context.Background())

				Expect(err).NotTo(HaveOccurred())
				Expect(len(services[0].Plans)).To(Equal(1))
				Expect(services[0].Plans[0].ID).To(Equal(DefaultPlanID))
				Expect(services[0].Plans[0].Name).To(Equal(DefaultPlanName))
			})
		})

		Context("when an instance is created", func() {
			It("returns a spec", func() {
				spec, err := testBroker.Provision(
					context.Background(),
					DefaultInstanceID,
					DefaultProvisionDetails(),
					true,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(spec.IsAsync).To(Equal(false))
			})

			It("creates a pvc", func() {
				_, err := testBroker.Provision(
					context.Background(),
					DefaultInstanceID,
					DefaultProvisionDetails(),
					true,
				)
				Expect(err).NotTo(HaveOccurred())

				pvcList, err := kubeClient.CoreV1().PersistentVolumeClaims(DefaultNamespace).List(context.TODO(), metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())

				Expect(len(pvcList.Items)).To(Equal(1))
				Expect(pvcList.Items[0].Name).To(Equal(DefaultInstanceID))
				Expect(*pvcList.Items[0].Spec.StorageClassName).To(Equal(DefaultStorageClass))

			})

			It("it's returned using GetInstance", func() {
				_, err := testBroker.Provision(
					context.Background(),
					DefaultInstanceID,
					DefaultProvisionDetails(),
					true,
				)
				Expect(err).NotTo(HaveOccurred())

				instance, err := testBroker.GetInstance(
					context.Background(),
					DefaultInstanceID,
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(instance.PlanID).To(Equal(DefaultPlanID))
				Expect(instance.ServiceID).To(Equal(DefaultServiceID))
			})

			Context("plan doesn't exist", func() {
				var provisionDetails brokerapi.ProvisionDetails

				BeforeEach(func() {
					provisionDetails = DefaultProvisionDetails()
					provisionDetails.PlanID = "foo"
				})

				It("returns an error", func() {
					_, err := testBroker.Provision(
						context.Background(),
						DefaultInstanceID,
						provisionDetails,
						true,
					)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("plan_id not recognized"))
				})
			})

			Context("instance already exists", func() {

				BeforeEach(func() {
					_, err := testBroker.Provision(
						context.Background(),
						DefaultInstanceID,
						DefaultProvisionDetails(),
						true,
					)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns an error", func() {
					_, err := testBroker.Provision(
						context.Background(),
						DefaultInstanceID,
						DefaultProvisionDetails(),
						true,
					)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("instance already exists"))
				})
			})
		})

		Context("when an instance is deleted", func() {
			BeforeEach(func() {
				_, err := testBroker.Provision(
					context.Background(),
					DefaultInstanceID,
					DefaultProvisionDetails(),
					true,
				)
				Expect(err).NotTo(HaveOccurred())
			})

			It("removes the pvc", func() {
				_, err := testBroker.Deprovision(
					context.Background(),
					DefaultInstanceID,
					DefaultDeprovisionDetails(),
					true,
				)
				Expect(err).NotTo(HaveOccurred())

				pvcList, err := kubeClient.CoreV1().PersistentVolumeClaims(DefaultNamespace).List(context.TODO(), metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(pvcList.Items)).To(Equal(0))
			})

			Context("if the instance doesn't exist", func() {
				It("returns an error", func() {
					_, err := testBroker.Deprovision(
						context.Background(),
						"foo",
						DefaultDeprovisionDetails(),
						true,
					)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("instance does not exist"))

					pvcList, err := kubeClient.CoreV1().PersistentVolumeClaims(DefaultNamespace).List(context.TODO(), metav1.ListOptions{})
					Expect(err).NotTo(HaveOccurred())
					Expect(len(pvcList.Items)).To(Equal(1))
				})
			})
		})

	})

	Describe("Binding", func() {
		BeforeEach(func() {
			_, err := testBroker.Provision(
				context.Background(),
				DefaultInstanceID,
				DefaultProvisionDetails(),
				true,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the service instance exists", func() {
			var (
				binding brokerapi.Binding
			)
			BeforeEach(func() {
				var err error
				binding, err = testBroker.Bind(
					context.Background(),
					DefaultInstanceID,
					DefaultBindingID,
					DefaultBindDetails(),
					true,
				)

				Expect(err).NotTo(HaveOccurred())
			})

			It("adds an annotation to the pvc", func() {
				pvcList, err := kubeClient.CoreV1().PersistentVolumeClaims(DefaultNamespace).List(context.TODO(), metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(pvcList.Items)).To(Equal(1))

				pvc := pvcList.Items[0]
				Expect(pvc.Annotations).To(HaveKey(DefaultAnnotationKey))
				Expect(pvc.Annotations[DefaultAnnotationKey]).To(Equal(DefaultMountLocation))
			})

			It("returns a correct binding", func() {
				Expect(binding.VolumeMounts).To(BeEquivalentTo([]brokerapi.VolumeMount{
					{
						Driver:       DefaultStorageClass,
						ContainerDir: DefaultMountLocation,
						Mode:         "rw",
						DeviceType:   "shared",
						Device: brokerapi.SharedDevice{
							VolumeId: DefaultInstanceID,
						},
					},
				}))
			})

			It("returns an existing binding", func() {
				bindingSpec, err := testBroker.GetBinding(
					context.Background(),
					DefaultInstanceID,
					DefaultBindingID,
				)
				Expect(err).NotTo(HaveOccurred())

				Expect(bindingSpec.VolumeMounts).To(BeEquivalentTo(binding.VolumeMounts))
			})

			It("deletes an existing binding", func() {
				_, err := testBroker.Unbind(
					context.Background(),
					DefaultInstanceID,
					DefaultBindingID,
					DefaultUnbindDetails(),
					true,
				)
				Expect(err).NotTo(HaveOccurred())

				pvcList, err := kubeClient.CoreV1().PersistentVolumeClaims(DefaultNamespace).List(context.TODO(), metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(pvcList.Items)).To(Equal(1))

				pvc := pvcList.Items[0]
				Expect(pvc.Annotations).ToNot(HaveKey(DefaultAnnotationKey))
			})

			Context("when the binding doesn't exist", func() {
				It("unbinding returns an error", func() {
					_, err := testBroker.Unbind(
						context.Background(),
						DefaultInstanceID,
						"foo",
						DefaultUnbindDetails(),
						true,
					)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("binding does not exist"))

					pvcList, err := kubeClient.CoreV1().PersistentVolumeClaims(DefaultNamespace).List(context.TODO(), metav1.ListOptions{})
					Expect(err).NotTo(HaveOccurred())
					Expect(len(pvcList.Items)).To(Equal(1))

					pvc := pvcList.Items[0]
					Expect(pvc.Annotations).To(HaveKey(DefaultAnnotationKey))
				})
			})

			Context("when the binding already exists", func() {
				It("returns an error", func() {
					_, err := testBroker.Bind(
						context.Background(),
						DefaultInstanceID,
						DefaultBindingID,
						DefaultBindDetails(),
						true,
					)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("binding already exists"))
				})
			})
		})

		Context("when the service instance doesn't exist", func() {
			It("binding returns an error", func() {
				_, err := testBroker.Bind(
					context.Background(),
					"foo",
					DefaultBindingID,
					DefaultBindDetails(),
					true,
				)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("instance does not exist"))
			})

			It("unbinding returns an error", func() {
				_, err := testBroker.Unbind(
					context.Background(),
					"foo",
					DefaultBindingID,
					DefaultUnbindDetails(),
					true,
				)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("instance does not exist"))
			})
		})
	})
})
