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
	"reflect"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/k14s/starlark-go/starlark"
	"github.com/pkg/errors"
	"github.com/wonderix/shalm/pkg/shalm"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	shalmv1a2 "github.com/wonderix/shalm/api/v1alpha2"
)

var (
	myFinalizerName   = "controller.shalm.wonderix.github.com"
	applyStatus       = "apply"
	applyErrorStatus  = "apply-error"
	deleteStatus      = "delete"
	deleteErrorStatus = "delete-error"
)

// ShalmChartReconciler reconciles a ShalmChart object
type ShalmChartReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Repo     shalm.Repo
	K8s      func(configs ...shalm.K8sConfig) (shalm.K8s, error)
	Load     func(thread *starlark.Thread, module string) (dict starlark.StringDict, err error)
	Recorder record.EventRecorder
}

type shalmChartPredicate struct {
}

// +kubebuilder:rbac:groups=shalm.wonderix.github.com,resources=shalmcharts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=shalm.wonderix.github.com,resources=shalmcharts/status,verbs=get;update;patch

// Reconcile -
func (r *ShalmChartReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	result := ctrl.Result{}
	ctx := context.Background()
	_ = r.Log.WithValues("shalmchart", req.NamespacedName)

	var shalmChart shalmv1a2.ShalmChart
	err := r.Client.Get(ctx, client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, &shalmChart)
	if err != nil {
		return result, client.IgnoreNotFound(err)
	}
	if shalmChart.ObjectMeta.DeletionTimestamp.IsZero() {
		if !containsString(shalmChart.ObjectMeta.Finalizers, myFinalizerName) {
			shalmChart.ObjectMeta.Finalizers = append(shalmChart.ObjectMeta.Finalizers, myFinalizerName)
			if err := r.Update(context.Background(), &shalmChart); err != nil {
				return result, errors.Wrapf(err, "error updating ShalmChart %s", req.NamespacedName.String())
			}
			shalmChart.Status.LastOp = shalmv1a2.Operation{Type: applyStatus, Progress: 0}
			if err := r.Status().Update(context.Background(), &shalmChart); err != nil {
				return result, errors.Wrapf(err, "error updating status of ShalmChart %s", req.NamespacedName.String())
			}
		}
		if err := r.apply(&shalmChart.Spec, func(progress int) {
			shalmChart.Status.LastOp = shalmv1a2.Operation{Type: applyStatus, Progress: progress}
			r.Status().Update(context.Background(), &shalmChart)
		}); err != nil {
			err = errors.Wrapf(err, "error applying ShalmChart %s", req.NamespacedName.String())
			r.Recorder.Event(&shalmChart, corev1.EventTypeWarning, "ApplyError", err.Error())
			shalmChart.Status.LastOp = shalmv1a2.Operation{Type: applyErrorStatus, Progress: 0}
			r.Status().Update(context.Background(), &shalmChart)
			return result, err
		}
		return result, err
	}
	if len(shalmChart.ObjectMeta.Finalizers) == 1 && containsString(shalmChart.ObjectMeta.Finalizers, myFinalizerName) {
		shalmChart.Status.LastOp = shalmv1a2.Operation{Type: deleteStatus, Progress: 0}
		if err := r.Status().Update(context.Background(), &shalmChart); err != nil {
			return result, errors.Wrapf(err, "error updating status of ShalmChart %s", req.NamespacedName.String())
		}
		if err := r.delete(&shalmChart.Spec, func(progress int) {
			shalmChart.Status.LastOp = shalmv1a2.Operation{Type: deleteStatus, Progress: progress}
			r.Status().Update(context.Background(), &shalmChart)
		}); err != nil {
			err = errors.Wrapf(err, "error deleting ShalmChart %s", req.NamespacedName.String())
			r.Recorder.Event(&shalmChart, corev1.EventTypeWarning, "DeleteError", err.Error())
			shalmChart.Status.LastOp = shalmv1a2.Operation{Type: deleteErrorStatus, Progress: 0}
			r.Status().Update(context.Background(), &shalmChart)
			return result, err
		}

		shalmChart.ObjectMeta.Finalizers = removeString(shalmChart.ObjectMeta.Finalizers, myFinalizerName)
		if err := r.Update(context.Background(), &shalmChart); err != nil {
			return result, errors.Wrapf(err, "error updating ShalmChart %s", req.NamespacedName.String())
		}
	}

	return result, err

}

func (r *ShalmChartReconciler) apply(spec *shalmv1a2.ChartSpec, progressCb shalm.ProgressSubscription) error {
	var tool shalm.Tool
	if err := tool.Set(spec.Tool); err != nil {
		return err
	}
	k8s, err := r.K8s(shalm.WithKubeConfigContent(spec.KubeConfig),
		shalm.WithProgressSubscription(progressCb),
		shalm.WithTool(tool))
	if err != nil {
		return err
	}
	thread := &starlark.Thread{Name: "main", Load: r.Load}
	chart, err := r.Repo.GetFromSpec(thread, spec)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()
	return chart.Apply(thread, k8s.WithContext(ctx))
}

func (r *ShalmChartReconciler) delete(spec *shalmv1a2.ChartSpec, progressCb shalm.ProgressSubscription) error {
	var tool shalm.Tool
	if err := tool.Set(spec.Tool); err != nil {
		return err
	}
	k8s, err := r.K8s(shalm.WithKubeConfigContent(spec.KubeConfig),
		shalm.WithProgressSubscription(progressCb),
		shalm.WithTool(tool))
	if err != nil {
		return err
	}
	thread := &starlark.Thread{Name: "main", Load: r.Load}
	chart, err := r.Repo.GetFromSpec(thread, spec)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()
	return chart.Delete(thread, k8s.WithContext(ctx), &shalm.DeleteOptions{})
}

// SetupWithManager -
func (r *ShalmChartReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&shalmv1a2.ShalmChart{}).
		WithEventFilter(&shalmChartPredicate{}).
		WithOptions(options).
		Complete(r)
}

// Create -
func (r *shalmChartPredicate) Create(event.CreateEvent) bool {
	return true
}

// Delete -
func (r *shalmChartPredicate) Delete(event.DeleteEvent) bool {
	return false
}

// Update -
func (r *shalmChartPredicate) Update(ev event.UpdateEvent) bool {
	old := ev.ObjectOld.(*shalmv1a2.ShalmChart)
	new := ev.ObjectNew.(*shalmv1a2.ShalmChart)
	if !reflect.DeepEqual(old.Spec, new.Spec) {
		return true
	}
	if !containsString(new.ObjectMeta.Finalizers, myFinalizerName) {
		return true
	}
	if !new.ObjectMeta.DeletionTimestamp.IsZero() {
		return true
	}
	return false
}

// Generic -
func (r *shalmChartPredicate) Generic(event.GenericEvent) bool {
	return false
}

// Helper functions to check and remove string from a slice of strings.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}
