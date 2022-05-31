/* cannot use https://github.com/prometheus/prometheus/blob/main/documentation/prometheus-mixin/mixin.libsonnet */
/* since it generates yaml with double quotes wrapped */

local rules = (
  import 'github.com/prometheus/alertmanager/doc/alertmanager-mixin/mixin.libsonnet'
).prometheusAlerts;

{
  _commonLabels:: {
    'app.kubernetes.io/component': 'operator',
    'app.kubernetes.io/name': 'observability-operator-alertmanager-rules',
    'app.kubernetes.io/part-of': 'observability-operator',
    prometheus: 'k8s',
    role: 'alert-rules',
  },

  rule: $.k.prometheusrule.new('observability-operator-alertmanager-rules', $._commonLabels, rules),
}
