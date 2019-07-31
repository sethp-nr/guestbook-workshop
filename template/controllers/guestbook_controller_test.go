/*

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

package controllers

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	webappv1 "guestbook-workshop/api/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("Guestbook Controller", func() {

	var (
		guestbook webappv1.GuestBook

		resources  []runtime.Object
		reconciler *GuestBookReconciler
		request    ctrl.Request
	)

	BeforeEach(func() {
		guestbook = webappv1.GuestBook{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-guestbook",
				Namespace: "default",
			},
			Spec: webappv1.GuestBookSpec{},
		}

		resources = []runtime.Object{
			&guestbook,
		}

		reconciler = &GuestBookReconciler{
			Client: k8sClient,
			Log:    logf.Log,
		}
	})

	JustBeforeEach(func() {
		for _, obj := range resources {
			Expect(k8sClient.Create(context.Background(), obj)).To(Succeed())
		}

		request = ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      guestbook.Name,
				Namespace: guestbook.Namespace,
			},
		}
	})

	AfterEach(func() {
		for _, obj := range resources {
			Expect(client.IgnoreNotFound(k8sClient.Delete(context.Background(), obj))).To(Succeed())
		}
		resources = nil
	})

	// This is using Gingko's "focus" mode to run only this one test.
	// Change this line to read `It("...", func() {` to run them all.
	FIt("creates a matching deployment", func() {
		_, err := reconciler.Reconcile(request)
		Expect(err).ToNot(HaveOccurred())

		deployments := appsv1.DeploymentList{}
		Expect(k8sClient.List(context.Background(), &deployments)).To(Succeed())

		Expect(deployments.Items).To(HaveLen(1))
		frontend := deployments.Items[0]
		resources = append(resources, &frontend) // for cleanup

		Expect(frontend.Name).To(Equal(guestbook.Name))
		Expect(frontend.Namespace).To(Equal(guestbook.Namespace))

		Expect(frontend.Spec.Replicas).ToNot(Equal(int32ptr(0)))
		Expect(frontend.Spec.Selector).To(SatisfyAny(
			Equal(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":  "guestbook",
					"tier": "frontend",
				},
			}),
			Equal(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"guestbook": guestbook.Name,
				},
			}),
		))

		podSpec := frontend.Spec.Template.Spec
		Expect(podSpec.Containers).To(HaveLen(1))
		Expect(podSpec.Containers[0].Image).To(Equal("gcr.io/google-samples/gb-frontend:v4"))
		Expect(podSpec.Containers[0].Env).To(ContainElement(
			corev1.EnvVar{
				Name:  "GET_HOSTS_FROM",
				Value: "dns",
			},
		))
		Expect(podSpec.Containers[0].Ports).To(ContainElement(
			corev1.ContainerPort{
				ContainerPort: 80,
				Protocol:      corev1.ProtocolTCP,
			},
		))
	})

	When("the spec has a replica count", func() {

		BeforeEach(func() {
			// TODO: uncomment this line
			// guestbook.Spec.Frontend.Replicas = int32ptr(5)
		})

		It("uses the specified number", func() {
			_, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			deployments := appsv1.DeploymentList{}
			Expect(k8sClient.List(context.Background(), &deployments)).To(Succeed())

			Expect(deployments.Items).To(HaveLen(1))
			frontend := deployments.Items[0]
			resources = append(resources, &frontend) // for cleanup

			Expect(frontend.Name).To(Equal(guestbook.Name))
			Expect(frontend.Namespace).To(Equal(guestbook.Namespace))

			Expect(frontend.Spec.Replicas).To(Equal(int32ptr(5)))
		})
	})

	// This is using Ginkgo's "pending" mode to skip this test.
	// If you'd like to take it on the "sevice" piece as a challenge,
	// change this line to `It("...", func(){` and see if it passes.
	PIt("creates a matching service", func() {
		_, err := reconciler.Reconcile(request)
		Expect(err).ToNot(HaveOccurred())

		services := corev1.ServiceList{}
		Expect(k8sClient.List(context.Background(), &services)).To(Succeed())

		Expect(services).To(HaveLen(2))
		var svc corev1.Service
		// Filter out the automatically populated `kubernetes` service
		for _, s := range services.Items {
			if s.Name != "kubernetes" {
				svc = s
				break
			}
		}
		resources = append(resources, &svc) // for cleanup

		Expect(svc.Name).To(Equal(guestbook.Name))
		Expect(svc.Namespace).To(Equal(guestbook.Namespace))
		Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeNodePort))
		Expect(svc.Spec.Ports).To(HaveLen(1))
		Expect(svc.Spec.Ports[0].Port).To(Equal(int32(80)))
		Expect(svc.Spec.Selector).To(SatisfyAny(
			Equal(map[string]string{
				"app":  "guestbook",
				"tier": "frontend",
			}),
			Equal(map[string]string{
				"guestbook": guestbook.Name,
			}),
		))
	})

	Context("with an existing deployment", func() {

		var (
			existing appsv1.Deployment
		)

		BeforeEach(func() {
			existing = appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      guestbook.Name,
					Namespace: guestbook.Namespace,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32ptr(0), // NB: We expect the default to be positive
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"guestbook": guestbook.Name,
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"guestbook": guestbook.Name,
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "frontend",
									Image: "gcr.io/google-samples/gb-frontend:v4",
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("100m"),
											corev1.ResourceMemory: resource.MustParse("100Mi"),
										},
									},
									Env: []corev1.EnvVar{
										{
											Name:  "GET_HOSTS_FROM",
											Value: "dns",
										},
									},
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: 80,
										},
									},
								},
							},
						},
					},
				},
			}

			resources = append(resources, &existing)
		})

		It("should update the replicas", func() {
			_, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			frontend := appsv1.Deployment{}
			Expect(k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      existing.Name,
				Namespace: existing.Namespace,
			}, &frontend)).To(Succeed())

			Expect(frontend.Spec.Replicas).ToNot(Equal(int32ptr(0)))
		})
	})
})

func int32ptr(i int32) *int32 {
	return &i
}
