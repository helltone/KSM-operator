// +build e2e

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	monitorv1alpha1 "github.com/example/k8s-state-monitor-operator/api/v1alpha1"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

var _ = Describe("StateMonitor E2E Tests", func() {
	const (
		timeout  = time.Minute * 2
		interval = time.Second * 5
	)

	var (
		ctx    context.Context
		k8sClient client.Client
	)

	BeforeEach(func() {
		ctx = context.Background()

		cfg, err := config.GetConfig()
		Expect(err).NotTo(HaveOccurred())

		err = monitorv1alpha1.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
		Expect(err).NotTo(HaveOccurred())
	})

	Context("When deploying the operator", func() {
		It("should manage monitor pods based on node count", func() {
			By("Creating a StateMonitor resource")
			sm := &monitorv1alpha1.StateMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name: "e2e-test-monitor",
				},
				Spec: monitorv1alpha1.StateMonitorSpec{
					NodesPerMonitor: 5,
					MonitorImage:    "busybox:latest",
					Namespace:       "monitoring",
					Resources: monitorv1alpha1.ResourceRequirements{
						Requests: monitorv1alpha1.ResourceList{
							CPU:    "10m",
							Memory: "32Mi",
						},
						Limits: monitorv1alpha1.ResourceList{
							CPU:    "50m",
							Memory: "64Mi",
						},
					},
				},
			}

			err := k8sClient.Create(ctx, sm)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for the StateMonitor to be reconciled")
			Eventually(func() bool {
				updatedSM := &monitorv1alpha1.StateMonitor{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: sm.Name}, updatedSM)
				if err != nil {
					return false
				}
				return updatedSM.Status.Phase != ""
			}, timeout, interval).Should(BeTrue())

			By("Verifying pods are created")
			Eventually(func() bool {
				podList := &corev1.PodList{}
				listOpts := []client.ListOption{
					client.InNamespace("monitoring"),
					client.MatchingLabels{
						"app.kubernetes.io/name":   "kube-state-metrics",
						"statemonitor.example.com": sm.Name,
					},
				}
				err := k8sClient.List(ctx, podList, listOpts...)
				if err != nil {
					return false
				}

				updatedSM := &monitorv1alpha1.StateMonitor{}
				err = k8sClient.Get(ctx, types.NamespacedName{Name: sm.Name}, updatedSM)
				if err != nil {
					return false
				}

				return len(podList.Items) == int(updatedSM.Status.DesiredReplicas)
			}, timeout, interval).Should(BeTrue())

			By("Cleaning up the StateMonitor")
			err = k8sClient.Delete(ctx, sm)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying pods are deleted")
			Eventually(func() int {
				podList := &corev1.PodList{}
				listOpts := []client.ListOption{
					client.InNamespace("monitoring"),
					client.MatchingLabels{
						"app.kubernetes.io/name":   "kube-state-metrics",
						"statemonitor.example.com": sm.Name,
					},
				}
				err := k8sClient.List(ctx, podList, listOpts...)
				if err != nil {
					return -1
				}
				return len(podList.Items)
			}, timeout, interval).Should(Equal(0))
		})

		It("should handle multiple StateMonitor resources", func() {
			By("Creating first StateMonitor")
			sm1 := &monitorv1alpha1.StateMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name: "e2e-monitor-1",
				},
				Spec: monitorv1alpha1.StateMonitorSpec{
					NodesPerMonitor: 10,
					MonitorImage:    "busybox:latest",
					Namespace:       "monitoring",
				},
			}
			err := k8sClient.Create(ctx, sm1)
			Expect(err).NotTo(HaveOccurred())

			By("Creating second StateMonitor")
			sm2 := &monitorv1alpha1.StateMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name: "e2e-monitor-2",
				},
				Spec: monitorv1alpha1.StateMonitorSpec{
					NodesPerMonitor: 15,
					MonitorImage:    "busybox:latest",
					Namespace:       "monitoring",
				},
			}
			err = k8sClient.Create(ctx, sm2)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for both monitors to be ready")
			Eventually(func() bool {
				sm1Updated := &monitorv1alpha1.StateMonitor{}
				sm2Updated := &monitorv1alpha1.StateMonitor{}
				
				err1 := k8sClient.Get(ctx, types.NamespacedName{Name: sm1.Name}, sm1Updated)
				err2 := k8sClient.Get(ctx, types.NamespacedName{Name: sm2.Name}, sm2Updated)
				
				if err1 != nil || err2 != nil {
					return false
				}
				
				return sm1Updated.Status.Phase == "Ready" && sm2Updated.Status.Phase == "Ready"
			}, timeout, interval).Should(BeTrue())

			By("Verifying each monitor has its own pods")
			podList1 := &corev1.PodList{}
			listOpts1 := []client.ListOption{
				client.InNamespace("monitoring"),
				client.MatchingLabels{
					"statemonitor.example.com": sm1.Name,
				},
			}
			err = k8sClient.List(ctx, podList1, listOpts1...)
			Expect(err).NotTo(HaveOccurred())

			podList2 := &corev1.PodList{}
			listOpts2 := []client.ListOption{
				client.InNamespace("monitoring"),
				client.MatchingLabels{
					"statemonitor.example.com": sm2.Name,
				},
			}
			err = k8sClient.List(ctx, podList2, listOpts2...)
			Expect(err).NotTo(HaveOccurred())

			Expect(podList1.Items).ToNot(BeEmpty())
			Expect(podList2.Items).ToNot(BeEmpty())

			By("Cleaning up")
			err = k8sClient.Delete(ctx, sm1)
			Expect(err).NotTo(HaveOccurred())
			err = k8sClient.Delete(ctx, sm2)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})