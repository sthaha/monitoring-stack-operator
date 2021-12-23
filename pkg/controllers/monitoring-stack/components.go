package monitoringstack

import (
	"fmt"

	policyv1 "k8s.io/api/policy/v1"

	stack "github.com/rhobs/monitoring-stack-operator/pkg/apis/v1alpha1"
	goctrl "github.com/rhobs/monitoring-stack-operator/pkg/controllers/grafana-operator"

	"k8s.io/apimachinery/pkg/util/intstr"

	"k8s.io/apimachinery/pkg/runtime"

	grafanav1alpha1 "github.com/grafana-operator/grafana-operator/v4/api/integreatly/v1alpha1"

	monv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const AdditionalScrapeConfigsSelfScrapeKey = "self-scrape-config"

type emptyObjectFunc func() client.Object

type patchObjectFunc func(existing client.Object) (client.Object, error)

type objectPatcher struct {
	empty emptyObjectFunc
	patch patchObjectFunc
}

type ObjectTypeError struct {
	expectedType string
	actualType   string
}

func NewObjectTypeError(expected runtime.Object, actual runtime.Object) error {
	gvk := func(o runtime.Object) string {
		gvk := o.GetObjectKind().GroupVersionKind()
		return gvk.Group + "/" + gvk.Version + " " + gvk.Kind
	}

	return ObjectTypeError{expectedType: gvk(expected), actualType: gvk(actual)}
}

func (e ObjectTypeError) Error() string {
	return fmt.Sprintf("object type %q is not %q", e.actualType, e.expectedType)
}

func stackComponentPatchers(ms *stack.MonitoringStack, instanceSelectorKey string, instanceSelectorValue string) ([]objectPatcher, error) {
	prometheusRBACResourceName := ms.Name + "-prometheus"
	alertmanagerRBACResourceName := ms.Name + "-alertmanager"

	rbacVerbs := []string{"get", "list", "watch"}
	additionalScrapeConfigsSecretName := ms.Name + "-prometheus-additional-scrape-configs"

	return []objectPatcher{
		{
			empty: func() client.Object {
				sa := newServiceAccount(prometheusRBACResourceName, ms.Namespace)
				return &corev1.ServiceAccount{
					TypeMeta:   sa.TypeMeta,
					ObjectMeta: sa.ObjectMeta,
				}
			},
			patch: func(existing client.Object) (client.Object, error) {
				return newServiceAccount(prometheusRBACResourceName, ms.Namespace), nil
			},
		},
		{
			empty: func() client.Object {
				role := newPrometheusRole(ms, prometheusRBACResourceName, rbacVerbs)
				return &rbacv1.Role{
					TypeMeta:   role.TypeMeta,
					ObjectMeta: role.ObjectMeta,
				}
			},
			patch: func(existing client.Object) (client.Object, error) {
				role := newPrometheusRole(ms, prometheusRBACResourceName, rbacVerbs)

				if existing == nil {
					return role, nil
				}

				desired, ok := existing.(*rbacv1.Role)
				if !ok {
					return nil, NewObjectTypeError(role, existing)
				}

				desired.Labels = role.Labels
				desired.Rules = role.Rules
				return desired, nil
			},
		},
		{
			empty: func() client.Object {
				rb := newRoleBinding(ms, prometheusRBACResourceName)
				return &rbacv1.RoleBinding{
					TypeMeta:   rb.TypeMeta,
					ObjectMeta: rb.ObjectMeta,
				}
			},
			patch: func(existing client.Object) (client.Object, error) {
				roleBinding := newRoleBinding(ms, prometheusRBACResourceName)

				if existing == nil {
					return roleBinding, nil
				}

				desired, ok := existing.(*rbacv1.RoleBinding)
				if !ok {
					return nil, NewObjectTypeError(roleBinding, existing)
				}

				desired.Labels = roleBinding.Labels
				desired.Subjects = roleBinding.Subjects
				desired.RoleRef = roleBinding.RoleRef
				return desired, nil
			},
		},
		{
			empty: func() client.Object {
				secret := newAdditionalScrapeConfigsSecret(ms, additionalScrapeConfigsSecretName)
				return &corev1.Secret{
					TypeMeta:   secret.TypeMeta,
					ObjectMeta: secret.ObjectMeta,
				}
			},
			patch: func(existing client.Object) (client.Object, error) {
				secret := newAdditionalScrapeConfigsSecret(ms, additionalScrapeConfigsSecretName)
				if existing == nil {
					return secret, nil
				}

				desired, ok := existing.(*corev1.Secret)
				if !ok {
					return nil, NewObjectTypeError(secret, existing)
				}

				desired.Labels = secret.Labels
				desired.StringData = secret.StringData

				return desired, nil
			},
		},
		{
			empty: func() client.Object {
				sa := newServiceAccount(alertmanagerRBACResourceName, ms.Namespace)
				return &corev1.ServiceAccount{
					TypeMeta:   sa.TypeMeta,
					ObjectMeta: sa.ObjectMeta,
				}
			},
			patch: func(existing client.Object) (client.Object, error) {
				return newServiceAccount(alertmanagerRBACResourceName, ms.Namespace), nil
			},
		},
		{
			empty: func() client.Object {
				alertmanager := newAlertmanager(ms, alertmanagerRBACResourceName, instanceSelectorKey, instanceSelectorValue)
				return &monv1.Alertmanager{
					TypeMeta:   alertmanager.TypeMeta,
					ObjectMeta: alertmanager.ObjectMeta,
				}
			},
			patch: func(existing client.Object) (client.Object, error) {
				alertmanager := newAlertmanager(ms, alertmanagerRBACResourceName, instanceSelectorKey, instanceSelectorValue)

				if existing == nil {
					return alertmanager, nil
				}

				desired, ok := existing.(*monv1.Alertmanager)
				if !ok {
					return nil, NewObjectTypeError(alertmanager, existing)
				}

				desired.Labels = alertmanager.Labels
				desired.Spec = alertmanager.Spec
				return desired, nil
			},
		},
		{
			empty: func() client.Object {
				service := newAlertmanagerService(ms, instanceSelectorKey, instanceSelectorValue)
				return &corev1.Service{
					TypeMeta:   service.TypeMeta,
					ObjectMeta: service.ObjectMeta,
				}
			},
			patch: func(existing client.Object) (client.Object, error) {
				service := newAlertmanagerService(ms, instanceSelectorKey, instanceSelectorValue)

				if existing == nil {
					return service, nil
				}

				desired, ok := existing.(*corev1.Service)
				if !ok {
					return nil, NewObjectTypeError(service, existing)
				}

				// The ClusterIP field is immutable and we have to take it from the observed object.
				service.Spec.ClusterIP = desired.Spec.ClusterIP
				desired.Spec = service.Spec
				desired.Labels = service.Labels
				return desired, nil
			},
		},
		{
			empty: func() client.Object {
				pdb := newAlertmanagerPDB(ms, instanceSelectorKey, instanceSelectorValue)
				return &policyv1.PodDisruptionBudget{
					TypeMeta:   pdb.TypeMeta,
					ObjectMeta: pdb.ObjectMeta,
				}
			},
			patch: func(existing client.Object) (client.Object, error) {
				pdb := newAlertmanagerPDB(ms, instanceSelectorKey, instanceSelectorValue)

				if existing == nil {
					return pdb, nil
				}

				desired, ok := existing.(*policyv1.PodDisruptionBudget)
				if !ok {
					return nil, NewObjectTypeError(pdb, existing)
				}

				desired.Spec = pdb.Spec
				desired.Labels = pdb.Labels
				return desired, nil
			},
		},
		{
			empty: func() client.Object {
				prometheus := newPrometheus(ms, prometheusRBACResourceName, additionalScrapeConfigsSecretName, instanceSelectorKey, instanceSelectorValue)
				return &monv1.Prometheus{
					TypeMeta:   prometheus.TypeMeta,
					ObjectMeta: prometheus.ObjectMeta,
				}
			},
			patch: func(existing client.Object) (client.Object, error) {
				prometheus := newPrometheus(ms, prometheusRBACResourceName, additionalScrapeConfigsSecretName, instanceSelectorKey, instanceSelectorValue)

				if existing == nil {
					return prometheus, nil
				}

				desired, ok := existing.(*monv1.Prometheus)
				if !ok {
					return nil, NewObjectTypeError(prometheus, existing)
				}

				desired.Labels = prometheus.Labels
				desired.Spec = prometheus.Spec
				return desired, nil
			},
		},
		{
			empty: func() client.Object {
				service := newPrometheusService(ms, instanceSelectorKey, instanceSelectorValue)
				return &corev1.Service{
					TypeMeta:   service.TypeMeta,
					ObjectMeta: service.ObjectMeta,
				}
			},
			patch: func(existing client.Object) (client.Object, error) {
				service := newPrometheusService(ms, instanceSelectorKey, instanceSelectorValue)

				if existing == nil {
					return service, nil
				}

				desired, ok := existing.(*corev1.Service)
				if !ok {
					return nil, NewObjectTypeError(service, existing)
				}

				// The ClusterIP field is immutable and we have to take it from the observed object.
				service.Spec.ClusterIP = desired.Spec.ClusterIP
				desired.Spec = service.Spec
				desired.Labels = service.Labels
				return desired, nil
			},
		},
		{
			empty: func() client.Object {
				dataSource := newGrafanaDataSource(ms)
				return &grafanav1alpha1.GrafanaDataSource{
					TypeMeta:   dataSource.TypeMeta,
					ObjectMeta: dataSource.ObjectMeta,
				}
			},
			patch: func(existing client.Object) (client.Object, error) {
				dataSource := newGrafanaDataSource(ms)
				if existing == nil {
					return dataSource, nil
				}

				desired, ok := existing.(*grafanav1alpha1.GrafanaDataSource)
				if !ok {
					return nil, NewObjectTypeError(dataSource, existing)
				}

				desired.Labels = dataSource.Labels
				desired.Spec = dataSource.Spec
				return desired, nil
			},
		},
	}, nil
}

func newGrafanaDataSource(ms *stack.MonitoringStack) *grafanav1alpha1.GrafanaDataSource {
	datasourceName := grafanaDSName(ms)
	prometheusURL := fmt.Sprintf("%s-prometheus.%s:9090", ms.Name, ms.Namespace)
	annotations := map[string]string{
		grafanaDatasourceOwnerName:      ms.Name,
		grafanaDatasourceOwnerNamespace: ms.Namespace,
	}

	return &grafanav1alpha1.GrafanaDataSource{
		TypeMeta: metav1.TypeMeta{
			// NOTE: uses a different naming convention for SchemeGroupVersion
			APIVersion: grafanav1alpha1.GroupVersion.String(),
			Kind:       "GrafanaDataSource",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        datasourceName,
			Namespace:   goctrl.Namespace,
			Annotations: annotations,
		},
		Spec: grafanav1alpha1.GrafanaDataSourceSpec{
			Name: datasourceName,
			Datasources: []grafanav1alpha1.GrafanaDataSourceFields{
				{
					Name:    datasourceName,
					Type:    "prometheus",
					Access:  "proxy",
					Url:     prometheusURL,
					Version: 1,
				},
			},
		},
	}
}

func newPrometheusRole(ms *stack.MonitoringStack, rbacResourceName string, rbacVerbs []string) *rbacv1.Role {
	return &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbacResourceName,
			Namespace: ms.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"services", "endpoints", "pods"},
				Verbs:     rbacVerbs,
			},
			{
				APIGroups: []string{"extensions", "networking.k8s.io"},
				Resources: []string{"ingresses"},
				Verbs:     rbacVerbs,
			},
		},
	}
}

func newServiceAccount(name string, namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func newPrometheus(
	ms *stack.MonitoringStack,
	rbacResourceName string,
	additionalScrapeConfigsSecretName string,
	instanceSelectorKey string,
	instanceSelectorValue string,
) *monv1.Prometheus {
	prometheusSelector := ms.Spec.ResourceSelector
	if prometheusSelector == nil {
		prometheusSelector = &metav1.LabelSelector{}
	}
	prometheus := &monv1.Prometheus{
		TypeMeta: metav1.TypeMeta{
			APIVersion: monv1.SchemeGroupVersion.String(),
			Kind:       "Prometheus",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ms.Name,
			Namespace: ms.Namespace,
			Labels:    objectLabels(ms.Name, ms.Name, instanceSelectorKey, instanceSelectorValue),
		},

		Spec: monv1.PrometheusSpec{
			PodMetadata: &monv1.EmbeddedObjectMetadata{
				Labels: podLabels("prometheus", ms.Name),
			},

			// Prometheus does not use an Enum for LogLevel, so need to convert to string
			LogLevel: string(ms.Spec.LogLevel),

			Retention: ms.Spec.Retention,
			Resources: ms.Spec.Resources,

			ServiceAccountName: rbacResourceName,

			ServiceMonitorSelector:          prometheusSelector,
			ServiceMonitorNamespaceSelector: nil,
			PodMonitorSelector:              prometheusSelector,
			PodMonitorNamespaceSelector:     nil,
			RuleSelector:                    prometheusSelector,
			RuleNamespaceSelector:           nil,

			Alerting: &monv1.AlertingSpec{
				Alertmanagers: []monv1.AlertmanagerEndpoints{
					{
						APIVersion: "v2",
						Name:       ms.Name + "-alertmanager",
						Namespace:  ms.Namespace,
						Scheme:     "http",
						Port:       intstr.FromString("web"),
					},
				},
			},

			// Prometheus should be configured for self-scraping through a static job.
			// It avoids the need to synthesize a ServiceMonitor with labels that will match
			// what the user defines in the monitoring stacks's resourceSelector field.
			AdditionalScrapeConfigs: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: additionalScrapeConfigsSecretName,
				},
				Key: AdditionalScrapeConfigsSelfScrapeKey,
			},
		},
	}
	return prometheus
}

func newRoleBinding(ms *stack.MonitoringStack, rbacResourceName string) *rbacv1.RoleBinding {
	roleBinding := &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbacResourceName,
			Namespace: ms.Namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup:  corev1.SchemeGroupVersion.Group,
				Kind:      "ServiceAccount",
				Name:      rbacResourceName,
				Namespace: ms.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "Role",
			Name:     rbacResourceName,
		},
	}
	return roleBinding
}

func newPrometheusService(ms *stack.MonitoringStack, instanceSelectorKey string, instanceSelectorValue string) *corev1.Service {
	name := ms.Name + "-prometheus"
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ms.Namespace,
			Labels:    objectLabels(name, ms.Name, instanceSelectorKey, instanceSelectorValue),
		},
		Spec: corev1.ServiceSpec{
			Selector: podLabels("prometheus", ms.Name),
			Ports: []corev1.ServicePort{
				{
					Name:       "web",
					Port:       9090,
					TargetPort: intstr.FromInt(9090),
				},
			},
		},
	}
}

func newAdditionalScrapeConfigsSecret(ms *stack.MonitoringStack, name string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ms.Namespace,
		},
		StringData: map[string]string{
			AdditionalScrapeConfigsSelfScrapeKey: `
- job_name: prometheus-self
  honor_labels: true
  relabel_configs:
  - action: keep
    source_labels:
    - __meta_kubernetes_service_label_app_kubernetes_io_name
    regex: ` + ms.Name + `-prometheus
  - action: keep
    source_labels:
    - __meta_kubernetes_endpoint_port_name
    regex: web
  - source_labels:
    - __meta_kubernetes_namespace
    target_label: namespace
  - source_labels:
    - __meta_kubernetes_service_name
    target_label: service
  - source_labels:
    - __meta_kubernetes_pod_name
    target_label: pod
  - source_labels:
    - __meta_kubernetes_pod_container_name
    target_label: container
  - target_label: endpoint
    replacement: web
  kubernetes_sd_configs:
  - role: endpoints
    namespaces:
      names:
      - ` + ms.Namespace + `
- job_name: alertmanager-self
  honor_timestamps: true
  scrape_interval: 30s
  scrape_timeout: 10s
  metrics_path: /metrics
  scheme: http
  follow_redirects: true
  relabel_configs:
  - source_labels: 
    - __meta_kubernetes_service_label_app_kubernetes_io_name
    separator: ;
    regex: ` + ms.Name + `-alertmanager
    replacement: $1
    action: keep
  - source_labels: [__meta_kubernetes_endpoint_port_name]
    separator: ;
    regex: web
    replacement: $1
    action: keep
  - source_labels: [__meta_kubernetes_namespace]
    separator: ;
    regex: (.*)
    target_label: namespace
    replacement: $1
    action: replace
  - source_labels: [__meta_kubernetes_service_name]
    separator: ;
    regex: (.*)
    target_label: service
    replacement: $1
    action: replace
  - source_labels: [__meta_kubernetes_pod_name]
    separator: ;
    regex: (.*)
    target_label: pod
    replacement: $1
    action: replace
  - source_labels: [__meta_kubernetes_pod_container_name]
    separator: ;
    regex: (.*)
    target_label: container
    replacement: $1
    action: replace
  - separator: ;
    regex: (.*)
    target_label: endpoint
    replacement: web
    action: replace
  kubernetes_sd_configs:
  - role: endpoints
    kubeconfig_file: ""
    follow_redirects: true
    namespaces:
      names:
      - ` + ms.Namespace,
		},
	}
}

func objectLabels(name string, msName string, instanceSelectorKey string, instanceSelectorValue string) map[string]string {
	return map[string]string{
		instanceSelectorKey:         instanceSelectorValue,
		"app.kubernetes.io/name":    name,
		"app.kubernetes.io/part-of": msName,
	}
}

func podLabels(component string, msName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/component": component,
		"app.kubernetes.io/part-of":   msName,
	}
}
