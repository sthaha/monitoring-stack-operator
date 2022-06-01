/*
Copyright 2021.

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

package monitoringstack

import (
	"context"
	"fmt"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/controller"

	policyv1 "k8s.io/api/policy/v1"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	stack "github.com/rhobs/observability-operator/pkg/apis/monitoring/v1alpha1"

	"github.com/go-logr/logr"
	monv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

type reconciler struct {
	k8sClient             client.Client
	scheme                *runtime.Scheme
	logger                logr.Logger
	instanceSelectorKey   string
	instanceSelectorValue string
	grafanaDSWatchCreated bool
	controller            controller.Controller
}

// Options allows for controller options to be set
type Options struct {
	InstanceSelector string
}

// RBAC for managing monitoring stacks
//+kubebuilder:rbac:groups=monitoring.rhobs,resources=monitoringstacks,verbs=list;watch;create;update
//+kubebuilder:rbac:groups=monitoring.rhobs,resources=monitoringstacks/status,verbs=get;update

// RBAC for managing Prometheus Operator CRs
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=alertmanagers;prometheuses;servicemonitors,verbs=list;watch;create;update;delete;patch
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=list;watch;create;update;delete;patch
//+kubebuilder:rbac:groups="",resources=serviceaccounts;services;secrets,verbs=list;watch;create;update;delete;patch
//+kubebuilder:rbac:groups="policy",resources=poddisruptionbudgets,verbs=list;watch;create;update;delete;patch

// RBAC for delegating permissions to Prometheus
//+kubebuilder:rbac:groups="",resources=pods;services;endpoints,verbs=get;list;watch
//+kubebuilder:rbac:groups=extensions;networking.k8s.io,resources=ingresses,verbs=get;list;watch

// RegisterWithManager registers the controller with Manager
func RegisterWithManager(mgr ctrl.Manager, opts Options) error {
	split := strings.Split(opts.InstanceSelector, "=")
	if len(split) != 2 {
		return fmt.Errorf("invalid InstanceSelector: %s", opts.InstanceSelector)
	}

	r := &reconciler{
		k8sClient:             mgr.GetClient(),
		scheme:                mgr.GetScheme(),
		logger:                ctrl.Log.WithName("observability-operator"),
		instanceSelectorKey:   split[0],
		instanceSelectorValue: split[1],
		grafanaDSWatchCreated: false,
	}

	// We only want to trigger a reconciliation when the generation
	// of a child changes. Until we need to update our the status for our own objects,
	// we can save CPU cycles by avoiding reconciliations triggered by
	// child status changes.
	p := predicate.GenerationChangedPredicate{}

	ctrl, err := ctrl.NewControllerManagedBy(mgr).
		WithLogger(ctrl.Log).
		For(&stack.MonitoringStack{}).
		Owns(&monv1.Prometheus{}).WithEventFilter(p).
		Owns(&monv1.Alertmanager{}).WithEventFilter(p).
		Owns(&v1.Service{}).WithEventFilter(p).
		Owns(&v1.ServiceAccount{}).WithEventFilter(p).
		Owns(&rbacv1.Role{}).WithEventFilter(p).
		Owns(&rbacv1.RoleBinding{}).WithEventFilter(p).
		Owns(&monv1.ServiceMonitor{}).WithEventFilter(p).
		Owns(&policyv1.PodDisruptionBudget{}).WithEventFilter(p).
		Build(r)

	if err != nil {
		return err
	}
	r.controller = ctrl
	return nil
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.logger.WithValues("stack", req.NamespacedName)
	logger.Info("Reconciling monitoring stack")
	ms, err := r.getStack(ctx, req)
	if err != nil {
		// retry since some error has occured
		return ctrl.Result{}, err
	}
	if ms == nil {
		// no such monitoring stack, so stop here
		return ctrl.Result{}, nil
	}

	if !ms.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.V(6).Info("skipping reconcile since object is already schedule for deletion")
		return ctrl.Result{}, nil
	}

	reconcilers := stackComponentReconcilers(ms, r.instanceSelectorKey, r.instanceSelectorValue)
	for _, reconciler := range reconcilers {
		err := reconciler(ctx, r.k8sClient, r.scheme)
		// handle create / update errors that can happen due to a stale cache by
		// retrying after some time.
		if errors.IsAlreadyExists(err) || errors.IsConflict(err) {
			logger.V(3).Info("skipping reconcile error", "err", err)
			return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
		}
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *reconciler) getStack(ctx context.Context, req ctrl.Request) (*stack.MonitoringStack, error) {
	logger := r.logger.WithValues("stack", req.NamespacedName)

	ms := stack.MonitoringStack{}

	if err := r.k8sClient.Get(ctx, req.NamespacedName, &ms); err != nil {
		if errors.IsNotFound(err) {
			logger.V(3).Info("stack could not be found; may be marked for deletion")
			return nil, nil
		}
		logger.Error(err, "failed to get monitoring stack")
		return nil, err
	}

	return &ms, nil
}
