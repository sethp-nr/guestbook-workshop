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

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	webappv1 "guestbook-workshop/api/v1"
)

// GuestBookReconciler reconciles a GuestBook object
type GuestBookReconciler struct {
	client.Client
	Log logr.Logger
}

// +kubebuilder:rbac:groups=webapp.example.com,resources=guestbooks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=webapp.example.com,resources=guestbooks/status,verbs=get;update;patch

func (r *GuestBookReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("guestbook", req.NamespacedName)

	var guestbook webappv1.GuestBook
	err := r.Get(ctx, req.NamespacedName, &guestbook)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("Successfully retrieved Guestbook", "replicas", guestbook.Spec.Frontend.Replicas)

	replicas := int32(3)
	if guestbook.Spec.Frontend.Replicas != nil {
		replicas = *guestbook.Spec.Frontend.Replicas
	}

	deployment := appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      guestbook.Name,
		Namespace: guestbook.Namespace,
	}, &deployment)
	if err == nil {
		// The deployment already exists, so we update it
		deployment.Spec.Replicas = &replicas

		err = r.Update(ctx, &deployment)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	} else if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	deployment.ObjectMeta = metav1.ObjectMeta{
		Name:      guestbook.Name,
		Namespace: guestbook.Namespace,
	}
	deployment.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app":  "guestbook",
			"tier": "frontend",
		},
	}
	deployment.Spec.Replicas = &replicas
	deployment.Spec.Template.ObjectMeta.Labels = map[string]string{
		"app":  "guestbook",
		"tier": "frontend",
	}
	deployment.Spec.Template.Spec.Containers = make([]corev1.Container, 1)
	deployment.Spec.Template.Spec.Containers[0].Name = "frontend" // or php-redis, as long as it's not empty
	deployment.Spec.Template.Spec.Containers[0].Image = "gcr.io/google-samples/gb-frontend:v4"
	deployment.Spec.Template.Spec.Containers[0].Resources.Requests = corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("100m"),
		corev1.ResourceMemory: resource.MustParse("100Mi"),
	}
	deployment.Spec.Template.Spec.Containers[0].Env = make([]corev1.EnvVar, 1)
	deployment.Spec.Template.Spec.Containers[0].Env[0].Name = "GET_HOSTS_FROM"
	deployment.Spec.Template.Spec.Containers[0].Env[0].Value = "dns"
	deployment.Spec.Template.Spec.Containers[0].Ports = make([]corev1.ContainerPort, 1)
	deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = 80

	err = r.Create(ctx, &deployment)
	return ctrl.Result{}, err
}

func (r *GuestBookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webappv1.GuestBook{}).
		Complete(r)
}
