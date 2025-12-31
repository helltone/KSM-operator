package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	monitorv1alpha1 "github.com/example/ksm-scale-operator/api/v1alpha1"
)

func TestKSMScaleController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "KSMScale Controller Suite")
}

var _ = Describe("KSMScale Controller", func() {
	const (
		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When reconciling a KSMScale", func() {
		var (
			reconciler *KSMScaleReconciler
			ctx        context.Context
			ksm        *monitorv1alpha1.KSMScale
			nodes      []*corev1.Node
		)

		BeforeEach(func() {
			ctx = context.Background()
			monitorv1alpha1.AddToScheme(scheme.Scheme)

			ksm = &monitorv1alpha1.KSMScale{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-monitor",
				},
				Spec: monitorv1alpha1.KSMScaleSpec{
					NodesPerMonitor: 15,
					MonitorImage:    "test-image:latest",
					Namespace:       "monitoring",
				},
			}

			nodes = []*corev1.Node{}
		})

		It("should create 1 pod for 1-15 nodes", func() {
			for i := 0; i < 10; i++ {
				nodes = append(nodes, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("node-%d", i),
					},
				})
			}

			objs := []runtime.Object{ksm}
			for _, node := range nodes {
				objs = append(objs, node)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(objs...).
				WithStatusSubresource(&monitorv1alpha1.KSMScale{}).
				Build()

			reconciler = &KSMScaleReconciler{
				Client: fakeClient,
				Scheme: scheme.Scheme,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: ksm.Name,
				},
			}

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			podList := &corev1.PodList{}
			err = fakeClient.List(ctx, podList, client.InNamespace("monitoring"))
			Expect(err).NotTo(HaveOccurred())
			Expect(podList.Items).To(HaveLen(1))

			updatedSM := &monitorv1alpha1.KSMScale{}
			err = fakeClient.Get(ctx, types.NamespacedName{Name: ksm.Name}, updatedSM)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedSM.Status.NodeCount).To(Equal(int32(10)))
			Expect(updatedSM.Status.DesiredReplicas).To(Equal(int32(1)))
			// CurrentReplicas shows count before creation, so should be 0
			Expect(updatedSM.Status.CurrentReplicas).To(Equal(int32(0)))
			Expect(updatedSM.Status.Phase).To(Equal("Scaling"))
		})

		It("should create 2 pods for 16-30 nodes", func() {
			for i := 0; i < 20; i++ {
				nodes = append(nodes, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("node-%d", i),
					},
				})
			}

			objs := []runtime.Object{ksm}
			for _, node := range nodes {
				objs = append(objs, node)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(objs...).
				WithStatusSubresource(&monitorv1alpha1.KSMScale{}).
				Build()

			reconciler = &KSMScaleReconciler{
				Client: fakeClient,
				Scheme: scheme.Scheme,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: ksm.Name,
				},
			}

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			podList := &corev1.PodList{}
			err = fakeClient.List(ctx, podList, client.InNamespace("monitoring"))
			Expect(err).NotTo(HaveOccurred())
			Expect(podList.Items).To(HaveLen(2))

			updatedSM := &monitorv1alpha1.KSMScale{}
			err = fakeClient.Get(ctx, types.NamespacedName{Name: ksm.Name}, updatedSM)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedSM.Status.NodeCount).To(Equal(int32(20)))
			Expect(updatedSM.Status.DesiredReplicas).To(Equal(int32(2)))
			// CurrentReplicas shows count before creation, so should be 0
			Expect(updatedSM.Status.CurrentReplicas).To(Equal(int32(0)))
			Expect(updatedSM.Status.Phase).To(Equal("Scaling"))
		})

		It("should create 3 pods for 31-45 nodes", func() {
			for i := 0; i < 35; i++ {
				nodes = append(nodes, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("node-%d", i),
					},
				})
			}

			objs := []runtime.Object{ksm}
			for _, node := range nodes {
				objs = append(objs, node)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(objs...).
				WithStatusSubresource(&monitorv1alpha1.KSMScale{}).
				Build()

			reconciler = &KSMScaleReconciler{
				Client: fakeClient,
				Scheme: scheme.Scheme,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: ksm.Name,
				},
			}

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			podList := &corev1.PodList{}
			err = fakeClient.List(ctx, podList, client.InNamespace("monitoring"))
			Expect(err).NotTo(HaveOccurred())
			Expect(podList.Items).To(HaveLen(3))
		})

		It("should scale down pods when nodes are removed", func() {
			for i := 0; i < 30; i++ {
				nodes = append(nodes, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("node-%d", i),
					},
				})
			}

			existingPods := []runtime.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-state-metrics-test-monitor-0",
						Namespace: "monitoring",
						Labels: map[string]string{
							"app.kubernetes.io/name":  "kube-state-metrics",
							"ksmscale.example.com": "test-monitor",
							"index":                   "0",
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-state-metrics-test-monitor-1",
						Namespace: "monitoring",
						Labels: map[string]string{
							"app.kubernetes.io/name":  "kube-state-metrics",
							"ksmscale.example.com": "test-monitor",
							"index":                   "1",
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-state-metrics-test-monitor-2",
						Namespace: "monitoring",
						Labels: map[string]string{
							"app.kubernetes.io/name":  "kube-state-metrics",
							"ksmscale.example.com": "test-monitor",
							"index":                   "2",
						},
					},
				},
			}

			objs := []runtime.Object{ksm}
			objs = append(objs, existingPods...)
			for _, node := range nodes {
				objs = append(objs, node)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(objs...).
				WithStatusSubresource(&monitorv1alpha1.KSMScale{}).
				Build()

			reconciler = &KSMScaleReconciler{
				Client: fakeClient,
				Scheme: scheme.Scheme,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: ksm.Name,
				},
			}

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			podList := &corev1.PodList{}
			err = fakeClient.List(ctx, podList, client.InNamespace("monitoring"))
			Expect(err).NotTo(HaveOccurred())
			Expect(podList.Items).To(HaveLen(2))

			updatedSM := &monitorv1alpha1.KSMScale{}
			err = fakeClient.Get(ctx, types.NamespacedName{Name: ksm.Name}, updatedSM)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedSM.Status.NodeCount).To(Equal(int32(30)))
			Expect(updatedSM.Status.DesiredReplicas).To(Equal(int32(2)))
			// CurrentReplicas shows count before deletion (3 existing pods)
			Expect(updatedSM.Status.CurrentReplicas).To(Equal(int32(3)))
			Expect(updatedSM.Status.Phase).To(Equal("Scaling"))
		})

		It("should use custom nodesPerMonitor ratio", func() {
			ksm.Spec.NodesPerMonitor = 5

			for i := 0; i < 11; i++ {
				nodes = append(nodes, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("node-%d", i),
					},
				})
			}

			objs := []runtime.Object{ksm}
			for _, node := range nodes {
				objs = append(objs, node)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(objs...).
				WithStatusSubresource(&monitorv1alpha1.KSMScale{}).
				Build()

			reconciler = &KSMScaleReconciler{
				Client: fakeClient,
				Scheme: scheme.Scheme,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: ksm.Name,
				},
			}

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			podList := &corev1.PodList{}
			err = fakeClient.List(ctx, podList, client.InNamespace("monitoring"))
			Expect(err).NotTo(HaveOccurred())
			Expect(podList.Items).To(HaveLen(3))
		})

		It("should apply resource limits from spec", func() {
			ksm.Spec.Resources = monitorv1alpha1.ResourceRequirements{
				Requests: monitorv1alpha1.ResourceList{
					CPU:    "100m",
					Memory: "128Mi",
				},
				Limits: monitorv1alpha1.ResourceList{
					CPU:    "200m",
					Memory: "256Mi",
				},
			}

			nodes = append(nodes, &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-1",
				},
			})

			objs := []runtime.Object{ksm}
			for _, node := range nodes {
				objs = append(objs, node)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(objs...).
				WithStatusSubresource(&monitorv1alpha1.KSMScale{}).
				Build()

			reconciler = &KSMScaleReconciler{
				Client: fakeClient,
				Scheme: scheme.Scheme,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: ksm.Name,
				},
			}

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			podList := &corev1.PodList{}
			err = fakeClient.List(ctx, podList, client.InNamespace("monitoring"))
			Expect(err).NotTo(HaveOccurred())
			Expect(podList.Items).To(HaveLen(1))

			pod := &podList.Items[0]
			Expect(pod.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("100m")))
			Expect(pod.Spec.Containers[0].Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("128Mi")))
			Expect(pod.Spec.Containers[0].Resources.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse("200m")))
			Expect(pod.Spec.Containers[0].Resources.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse("256Mi")))
		})

		It("should handle zero nodes gracefully", func() {
			objs := []runtime.Object{ksm}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(objs...).
				WithStatusSubresource(&monitorv1alpha1.KSMScale{}).
				Build()

			reconciler = &KSMScaleReconciler{
				Client: fakeClient,
				Scheme: scheme.Scheme,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: ksm.Name,
				},
			}

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			podList := &corev1.PodList{}
			err = fakeClient.List(ctx, podList, client.InNamespace("monitoring"))
			Expect(err).NotTo(HaveOccurred())
			Expect(podList.Items).To(HaveLen(0))

			updatedSM := &monitorv1alpha1.KSMScale{}
			err = fakeClient.Get(ctx, types.NamespacedName{Name: ksm.Name}, updatedSM)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedSM.Status.NodeCount).To(Equal(int32(0)))
			Expect(updatedSM.Status.DesiredReplicas).To(Equal(int32(0)))
			Expect(updatedSM.Status.CurrentReplicas).To(Equal(int32(0)))
			Expect(updatedSM.Status.Phase).To(Equal("Ready"))
		})

		It("should return without error if KSMScale is not found", func() {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				Build()

			reconciler = &KSMScaleReconciler{
				Client: fakeClient,
				Scheme: scheme.Scheme,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: "non-existent",
				},
			}

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should handle missing pods with gaps in indexes", func() {
			for i := 0; i < 60; i++ {
				nodes = append(nodes, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("node-%d", i),
					},
				})
			}

			// Create only pods with indexes 1 and 3 (missing 0 and 2)
			existingPods := []runtime.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-state-metrics-test-monitor-1",
						Namespace: "monitoring",
						Labels: map[string]string{
							"app.kubernetes.io/name":  "kube-state-metrics",
							"ksmscale.example.com": "test-monitor",
							"index":                   "1",
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-state-metrics-test-monitor-3",
						Namespace: "monitoring",
						Labels: map[string]string{
							"app.kubernetes.io/name":  "kube-state-metrics",
							"ksmscale.example.com": "test-monitor",
							"index":                   "3",
						},
					},
				},
			}

			objs := []runtime.Object{ksm}
			objs = append(objs, existingPods...)
			for _, node := range nodes {
				objs = append(objs, node)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(objs...).
				WithStatusSubresource(&monitorv1alpha1.KSMScale{}).
				Build()

			reconciler = &KSMScaleReconciler{
				Client: fakeClient,
				Scheme: scheme.Scheme,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: ksm.Name,
				},
			}

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			// Should create missing pods at indexes 0 and 2 to fill gaps
			podList := &corev1.PodList{}
			err = fakeClient.List(ctx, podList, client.InNamespace("monitoring"))
			Expect(err).NotTo(HaveOccurred())
			Expect(podList.Items).To(HaveLen(4)) // 2 existing + 2 new = 4 total

			updatedSM := &monitorv1alpha1.KSMScale{}
			err = fakeClient.Get(ctx, types.NamespacedName{Name: ksm.Name}, updatedSM)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedSM.Status.NodeCount).To(Equal(int32(60)))
			Expect(updatedSM.Status.DesiredReplicas).To(Equal(int32(4))) // 60/15 = 4
			// CurrentReplicas shows 2 existing pods before creation of missing ones
			Expect(updatedSM.Status.CurrentReplicas).To(Equal(int32(2)))
			Expect(updatedSM.Status.Phase).To(Equal("Scaling"))
		})

		It("should scale up correctly when ratio changes", func() {
			for i := 0; i < 20; i++ {
				nodes = append(nodes, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("node-%d", i),
					},
				})
			}

			// Start with 5 nodes per monitor (20 nodes = 4 pods)
			ksm.Spec.NodesPerMonitor = 5

			existingPods := []runtime.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-state-metrics-test-monitor-0",
						Namespace: "monitoring",
						Labels: map[string]string{
							"app.kubernetes.io/name":  "kube-state-metrics",
							"ksmscale.example.com": "test-monitor",
							"index":                   "0",
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-state-metrics-test-monitor-1",
						Namespace: "monitoring",
						Labels: map[string]string{
							"app.kubernetes.io/name":  "kube-state-metrics",
							"ksmscale.example.com": "test-monitor",
							"index":                   "1",
						},
					},
				},
			}

			objs := []runtime.Object{ksm}
			objs = append(objs, existingPods...)
			for _, node := range nodes {
				objs = append(objs, node)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(objs...).
				WithStatusSubresource(&monitorv1alpha1.KSMScale{}).
				Build()

			reconciler = &KSMScaleReconciler{
				Client: fakeClient,
				Scheme: scheme.Scheme,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: ksm.Name,
				},
			}

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			// Should scale up from 2 to 4 pods (20 nodes / 5 = 4 pods)
			podList := &corev1.PodList{}
			err = fakeClient.List(ctx, podList, client.InNamespace("monitoring"))
			Expect(err).NotTo(HaveOccurred())
			Expect(podList.Items).To(HaveLen(4)) // 2 existing + 2 new = 4 total

			updatedSM := &monitorv1alpha1.KSMScale{}
			err = fakeClient.Get(ctx, types.NamespacedName{Name: ksm.Name}, updatedSM)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedSM.Status.NodeCount).To(Equal(int32(20)))
			Expect(updatedSM.Status.DesiredReplicas).To(Equal(int32(4)))
			// CurrentReplicas shows 2 existing pods before creation of missing ones
			Expect(updatedSM.Status.CurrentReplicas).To(Equal(int32(2)))
			Expect(updatedSM.Status.Phase).To(Equal("Scaling"))
		})

		It("should handle pod recreation when one pod is deleted", func() {
			for i := 0; i < 6; i++ {
				nodes = append(nodes, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("node-%d", i),
					},
				})
			}

			ksm.Spec.NodesPerMonitor = 2 // 6 nodes / 2 = 3 pods

			// Start with only pod-1 and pod-2 (pod-0 is missing)
			existingPods := []runtime.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-state-metrics-test-monitor-1",
						Namespace: "monitoring",
						Labels: map[string]string{
							"app.kubernetes.io/name":  "kube-state-metrics",
							"ksmscale.example.com": "test-monitor",
							"index":                   "1",
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-state-metrics-test-monitor-2",
						Namespace: "monitoring",
						Labels: map[string]string{
							"app.kubernetes.io/name":  "kube-state-metrics",
							"ksmscale.example.com": "test-monitor",
							"index":                   "2",
						},
					},
				},
			}

			objs := []runtime.Object{ksm}
			objs = append(objs, existingPods...)
			for _, node := range nodes {
				objs = append(objs, node)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(objs...).
				WithStatusSubresource(&monitorv1alpha1.KSMScale{}).
				Build()

			reconciler = &KSMScaleReconciler{
				Client: fakeClient,
				Scheme: scheme.Scheme,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: ksm.Name,
				},
			}

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			// Should create the missing pod-0
			podList := &corev1.PodList{}
			err = fakeClient.List(ctx, podList, client.InNamespace("monitoring"))
			Expect(err).NotTo(HaveOccurred())
			Expect(podList.Items).To(HaveLen(3)) // 2 existing + 1 new = 3 total

			updatedSM := &monitorv1alpha1.KSMScale{}
			err = fakeClient.Get(ctx, types.NamespacedName{Name: ksm.Name}, updatedSM)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedSM.Status.NodeCount).To(Equal(int32(6)))
			Expect(updatedSM.Status.DesiredReplicas).To(Equal(int32(3)))
			// CurrentReplicas shows 2 existing pods before creation of missing pod-0
			Expect(updatedSM.Status.CurrentReplicas).To(Equal(int32(2)))
			Expect(updatedSM.Status.Phase).To(Equal("Scaling"))
		})

		It("should scale down properly with correct pod selection", func() {
			for i := 0; i < 15; i++ {
				nodes = append(nodes, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("node-%d", i),
					},
				})
			}

			ksm.Spec.NodesPerMonitor = 3 // 15 nodes / 3 = 5 pods, but we have 6

			existingPods := []runtime.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-state-metrics-test-monitor-0",
						Namespace: "monitoring",
						Labels: map[string]string{
							"app.kubernetes.io/name":  "kube-state-metrics",
							"ksmscale.example.com": "test-monitor",
							"index":                   "0",
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-state-metrics-test-monitor-1",
						Namespace: "monitoring",
						Labels: map[string]string{
							"app.kubernetes.io/name":  "kube-state-metrics",
							"ksmscale.example.com": "test-monitor",
							"index":                   "1",
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-state-metrics-test-monitor-2",
						Namespace: "monitoring",
						Labels: map[string]string{
							"app.kubernetes.io/name":  "kube-state-metrics",
							"ksmscale.example.com": "test-monitor",
							"index":                   "2",
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-state-metrics-test-monitor-3",
						Namespace: "monitoring",
						Labels: map[string]string{
							"app.kubernetes.io/name":  "kube-state-metrics",
							"ksmscale.example.com": "test-monitor",
							"index":                   "3",
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-state-metrics-test-monitor-4",
						Namespace: "monitoring",
						Labels: map[string]string{
							"app.kubernetes.io/name":  "kube-state-metrics",
							"ksmscale.example.com": "test-monitor",
							"index":                   "4",
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-state-metrics-test-monitor-5",
						Namespace: "monitoring",
						Labels: map[string]string{
							"app.kubernetes.io/name":  "kube-state-metrics",
							"ksmscale.example.com": "test-monitor",
							"index":                   "5",
						},
					},
				},
			}

			objs := []runtime.Object{ksm}
			objs = append(objs, existingPods...)
			for _, node := range nodes {
				objs = append(objs, node)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(objs...).
				WithStatusSubresource(&monitorv1alpha1.KSMScale{}).
				Build()

			reconciler = &KSMScaleReconciler{
				Client: fakeClient,
				Scheme: scheme.Scheme,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: ksm.Name,
				},
			}

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			// Should scale down from 6 to 5 pods (15 nodes / 3 = 5 pods)
			podList := &corev1.PodList{}
			err = fakeClient.List(ctx, podList, client.InNamespace("monitoring"))
			Expect(err).NotTo(HaveOccurred())
			Expect(podList.Items).To(HaveLen(5)) // 6 - 1 = 5 remaining

			updatedSM := &monitorv1alpha1.KSMScale{}
			err = fakeClient.Get(ctx, types.NamespacedName{Name: ksm.Name}, updatedSM)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedSM.Status.NodeCount).To(Equal(int32(15)))
			Expect(updatedSM.Status.DesiredReplicas).To(Equal(int32(5)))
			// CurrentReplicas shows 6 existing pods before deletion of excess pod
			Expect(updatedSM.Status.CurrentReplicas).To(Equal(int32(6)))
			Expect(updatedSM.Status.Phase).To(Equal("Scaling"))
		})

		It("should handle dynamic ratio changes from large to small", func() {
			for i := 0; i < 12; i++ {
				nodes = append(nodes, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("node-%d", i),
					},
				})
			}

			ksm.Spec.NodesPerMonitor = 1 // Initially 1:1 ratio (12 pods)

			// Start with all 12 pods
			var existingPods []runtime.Object
			for i := 0; i < 12; i++ {
				existingPods = append(existingPods, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("kube-state-metrics-test-monitor-%d", i),
						Namespace: "monitoring",
						Labels: map[string]string{
							"app.kubernetes.io/name":  "kube-state-metrics",
							"ksmscale.example.com": "test-monitor",
							"index":                   fmt.Sprintf("%d", i),
						},
					},
				})
			}

			objs := []runtime.Object{ksm}
			objs = append(objs, existingPods...)
			for _, node := range nodes {
				objs = append(objs, node)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(objs...).
				WithStatusSubresource(&monitorv1alpha1.KSMScale{}).
				Build()

			reconciler = &KSMScaleReconciler{
				Client: fakeClient,
				Scheme: scheme.Scheme,
			}

			// First reconcile with 1:1 ratio - should keep all 12 pods
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: ksm.Name,
				},
			}

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			// Now change to 6:1 ratio (should scale down to 2 pods)
			updatedSM := &monitorv1alpha1.KSMScale{}
			err = fakeClient.Get(ctx, types.NamespacedName{Name: ksm.Name}, updatedSM)
			Expect(err).NotTo(HaveOccurred())

			updatedSM.Spec.NodesPerMonitor = 6
			err = fakeClient.Update(ctx, updatedSM)
			Expect(err).NotTo(HaveOccurred())

			// Reconcile again with new ratio
			_, err = reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			// Should scale down from 12 to 2 pods (12 nodes / 6 = 2 pods)
			podList := &corev1.PodList{}
			err = fakeClient.List(ctx, podList, client.InNamespace("monitoring"))
			Expect(err).NotTo(HaveOccurred())
			Expect(podList.Items).To(HaveLen(2)) // 12 - 10 = 2 remaining

			finalSM := &monitorv1alpha1.KSMScale{}
			err = fakeClient.Get(ctx, types.NamespacedName{Name: ksm.Name}, finalSM)
			Expect(err).NotTo(HaveOccurred())
			Expect(finalSM.Status.NodeCount).To(Equal(int32(12)))
			Expect(finalSM.Status.DesiredReplicas).To(Equal(int32(2)))
			// CurrentReplicas shows 12 existing pods before deletion of excess
			Expect(finalSM.Status.CurrentReplicas).To(Equal(int32(12)))
			Expect(finalSM.Status.Phase).To(Equal("Scaling"))
		})
	})

	Context("When reconciling sharded KSMScale", func() {
		var (
			reconciler *KSMScaleReconciler
			ctx        context.Context
			ksm        *monitorv1alpha1.KSMScale
			nodes      []*corev1.Node
		)

		BeforeEach(func() {
			ctx = context.Background()
			monitorv1alpha1.AddToScheme(scheme.Scheme)
			nodes = []*corev1.Node{}
		})

		It("should enforce minimum 3 pods for 3 shards with small cluster", func() {
			// Create 10 nodes (would normally need only 1 pod)
			for i := 0; i < 10; i++ {
				nodes = append(nodes, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("node-%d", i),
					},
				})
			}

			ksm = &monitorv1alpha1.KSMScale{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-sharded",
				},
				Spec: monitorv1alpha1.KSMScaleSpec{
					NodesPerMonitor: 15,
					MonitorImage:    "test-image:latest",
					Namespace:       "monitoring",
					Sharding: &monitorv1alpha1.ShardingConfig{
						Enabled:             true,
						ShardCount:          3,
						MinReplicasPerShard: 1,
					},
				},
			}

			objs := []runtime.Object{ksm}
			for _, node := range nodes {
				objs = append(objs, node)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(objs...).
				WithStatusSubresource(&monitorv1alpha1.KSMScale{}).
				Build()

			reconciler = &KSMScaleReconciler{
				Client: fakeClient,
				Scheme: scheme.Scheme,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: ksm.Name,
				},
			}

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			// Should create 3 pods (minimum for 3 shards)
			podList := &corev1.PodList{}
			err = fakeClient.List(ctx, podList, client.InNamespace("monitoring"))
			Expect(err).NotTo(HaveOccurred())
			Expect(podList.Items).To(HaveLen(3))

			// Verify sharding configuration
			for i, pod := range podList.Items {
				Expect(pod.Labels["shard-id"]).To(Equal(fmt.Sprintf("%d", i)))
				Expect(pod.Labels["total-shards"]).To(Equal("3"))
				Expect(pod.Labels["sharding-enabled"]).To(Equal("true"))
				
				// Check container args for sharding
				args := pod.Spec.Containers[0].Args
				Expect(args).To(ContainElement(fmt.Sprintf("--shard=%d", i)))
				Expect(args).To(ContainElement("--total-shards=3"))
			}

			// Verify status
			updatedSM := &monitorv1alpha1.KSMScale{}
			err = fakeClient.Get(ctx, types.NamespacedName{Name: ksm.Name}, updatedSM)
			Expect(err).NotTo(HaveOccurred())
			
			Expect(updatedSM.Status.NodeCount).To(Equal(int32(10)))
			Expect(updatedSM.Status.DesiredReplicas).To(Equal(int32(3))) // Enforced minimum
			// CurrentReplicas shows 0 existing pods before creation
			Expect(updatedSM.Status.CurrentReplicas).To(Equal(int32(0)))
			// Phase is "Incomplete" when activeShards < shardCount (0 < 3)
			Expect(updatedSM.Status.Phase).To(Equal("Incomplete"))
			
			// Verify sharding status
			Expect(updatedSM.Status.ShardingStatus).ToNot(BeNil())
			Expect(updatedSM.Status.ShardingStatus.TotalShards).To(Equal(int32(3)))
			// ActiveShards is 0 because no pods exist yet (ActiveShards counts pods with Ready > 0)
			Expect(updatedSM.Status.ShardingStatus.ActiveShards).To(Equal(int32(0)))
		})

		It("should distribute pods across shards with round-robin", func() {
			// Create 75 nodes (would need 5 pods with 15 nodes per monitor)
			for i := 0; i < 75; i++ {
				nodes = append(nodes, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("node-%d", i),
					},
				})
			}

			ksm = &monitorv1alpha1.KSMScale{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-distributed",
				},
				Spec: monitorv1alpha1.KSMScaleSpec{
					NodesPerMonitor: 15,
					MonitorImage:    "test-image:latest",
					Namespace:       "monitoring",
					Sharding: &monitorv1alpha1.ShardingConfig{
						Enabled:             true,
						ShardCount:          3,
						MinReplicasPerShard: 1,
					},
				},
			}

			objs := []runtime.Object{ksm}
			for _, node := range nodes {
				objs = append(objs, node)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(objs...).
				WithStatusSubresource(&monitorv1alpha1.KSMScale{}).
				Build()

			reconciler = &KSMScaleReconciler{
				Client: fakeClient,
				Scheme: scheme.Scheme,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: ksm.Name,
				},
			}

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			// Should create 5 pods distributed across 3 shards
			podList := &corev1.PodList{}
			err = fakeClient.List(ctx, podList, client.InNamespace("monitoring"))
			Expect(err).NotTo(HaveOccurred())
			Expect(podList.Items).To(HaveLen(5))

			// Verify round-robin distribution
			shardCounts := map[string]int{}
			for _, pod := range podList.Items {
				shardID := pod.Labels["shard-id"]
				shardCounts[shardID]++
			}

			// Round-robin distribution: pods 0,3 -> shard 0; pods 1,4 -> shard 1; pod 2 -> shard 2
			Expect(shardCounts["0"]).To(Equal(2))
			Expect(shardCounts["1"]).To(Equal(2))
			Expect(shardCounts["2"]).To(Equal(1))

			// Verify status shows correct desired replicas
			updatedSM := &monitorv1alpha1.KSMScale{}
			err = fakeClient.Get(ctx, types.NamespacedName{Name: ksm.Name}, updatedSM)
			Expect(err).NotTo(HaveOccurred())
			
			Expect(updatedSM.Status.DesiredReplicas).To(Equal(int32(5)))
			// CurrentReplicas shows 0 before creation
			Expect(updatedSM.Status.CurrentReplicas).To(Equal(int32(0)))
			
			// ShardingStatus should be populated
			Expect(updatedSM.Status.ShardingStatus).ToNot(BeNil())
			if updatedSM.Status.ShardingStatus != nil && updatedSM.Status.ShardingStatus.ShardDistribution != nil {
				// Distribution shows EXISTING pods, not planned. Since no pods exist yet, all should be 0
				Expect(updatedSM.Status.ShardingStatus.ShardDistribution["0"].Replicas).To(Equal(int32(0)))
				Expect(updatedSM.Status.ShardingStatus.ShardDistribution["1"].Replicas).To(Equal(int32(0)))
				Expect(updatedSM.Status.ShardingStatus.ShardDistribution["2"].Replicas).To(Equal(int32(0)))
			}
		})

		It("should create PodDisruptionBudget when enabled", func() {
			for i := 0; i < 10; i++ {
				nodes = append(nodes, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("node-%d", i),
					},
				})
			}

			ksm = &monitorv1alpha1.KSMScale{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pdb",
				},
				Spec: monitorv1alpha1.KSMScaleSpec{
					NodesPerMonitor: 15,
					MonitorImage:    "test-image:latest",
					Namespace:       "monitoring",
					Sharding: &monitorv1alpha1.ShardingConfig{
						Enabled:             true,
						ShardCount:          3,
						MinReplicasPerShard: 1,
						Distribution: &monitorv1alpha1.DistributionConfig{
							EnablePDB:      true,
							MaxUnavailable: 1,
						},
					},
				},
			}

			objs := []runtime.Object{ksm}
			for _, node := range nodes {
				objs = append(objs, node)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(objs...).
				WithStatusSubresource(&monitorv1alpha1.KSMScale{}).
				Build()

			reconciler = &KSMScaleReconciler{
				Client: fakeClient,
				Scheme: scheme.Scheme,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: ksm.Name,
				},
			}

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			// Verify PDB was created
			pdb := &policyv1.PodDisruptionBudget{}
			err = fakeClient.Get(ctx, types.NamespacedName{
				Name:      "ksm-test-pdb-shards",
				Namespace: "monitoring",
			}, pdb)
			Expect(err).NotTo(HaveOccurred())
			Expect(pdb.Spec.MaxUnavailable.IntVal).To(Equal(int32(1)))
		})

		It("should provide efficiency warnings", func() {
			// Create small cluster with too many shards
			for i := 0; i < 10; i++ {
				nodes = append(nodes, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("node-%d", i),
					},
				})
			}

			ksm = &monitorv1alpha1.KSMScale{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-warning",
				},
				Spec: monitorv1alpha1.KSMScaleSpec{
					NodesPerMonitor: 15,
					MonitorImage:    "test-image:latest",
					Namespace:       "monitoring",
					Sharding: &monitorv1alpha1.ShardingConfig{
						Enabled:             true,
						ShardCount:          5, // Too many for 10 nodes
						MinReplicasPerShard: 1,
					},
				},
			}

			objs := []runtime.Object{ksm}
			for _, node := range nodes {
				objs = append(objs, node)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(objs...).
				WithStatusSubresource(&monitorv1alpha1.KSMScale{}).
				Build()

			reconciler = &KSMScaleReconciler{
				Client: fakeClient,
				Scheme: scheme.Scheme,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: ksm.Name,
				},
			}

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			// Verify warning is set
			updatedSM := &monitorv1alpha1.KSMScale{}
			err = fakeClient.Get(ctx, types.NamespacedName{Name: ksm.Name}, updatedSM)
			Expect(err).NotTo(HaveOccurred())
			
			Expect(updatedSM.Status.ShardingStatus.Warning).ToNot(BeEmpty())
			Expect(updatedSM.Status.ShardingStatus.Warning).To(ContainSubstring("Over-sharded"))
		})

		It("should handle shard distribution calculation correctly", func() {
			reconciler = &KSMScaleReconciler{}

			// Test various pod/shard combinations
			By("Testing 5 pods across 3 shards")
			assignments := reconciler.calculateShardDistribution(5, 3)
			Expect(assignments[0]).To(Equal(int32(0))) // Pod 0 -> Shard 0
			Expect(assignments[1]).To(Equal(int32(1))) // Pod 1 -> Shard 1
			Expect(assignments[2]).To(Equal(int32(2))) // Pod 2 -> Shard 2
			Expect(assignments[3]).To(Equal(int32(0))) // Pod 3 -> Shard 0
			Expect(assignments[4]).To(Equal(int32(1))) // Pod 4 -> Shard 1

			By("Testing 6 pods across 3 shards")
			assignments = reconciler.calculateShardDistribution(6, 3)
			Expect(assignments[0]).To(Equal(int32(0))) // Pod 0 -> Shard 0
			Expect(assignments[1]).To(Equal(int32(1))) // Pod 1 -> Shard 1
			Expect(assignments[2]).To(Equal(int32(2))) // Pod 2 -> Shard 2
			Expect(assignments[3]).To(Equal(int32(0))) // Pod 3 -> Shard 0
			Expect(assignments[4]).To(Equal(int32(1))) // Pod 4 -> Shard 1
			Expect(assignments[5]).To(Equal(int32(2))) // Pod 5 -> Shard 2
		})
	})
})