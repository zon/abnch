{{/* The chart name. */}}
{{- define "abnchd.name" -}}
{{- .Chart.Name }}
{{- end }}

{{/* The release-scoped object name. */}}
{{- define "abnchd.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "abnchd.name" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/* Labels common to every object in the release. */}}
{{- define "abnchd.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
{{ include "abnchd.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/* Labels that select the release's pods. */}}
{{- define "abnchd.selectorLabels" -}}
app.kubernetes.io/name: {{ include "abnchd.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/* The Secret holding the service's secret config values. */}}
{{- define "abnchd.secretName" -}}
{{- .Values.secrets.name | default (include "abnchd.fullname" .) }}
{{- end }}

{{/* The ConfigMap holding the service config the chart renders. */}}
{{- define "abnchd.configMapName" -}}
{{- printf "%s-config" (include "abnchd.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/* The service config file the container reads. */}}
{{- define "abnchd.configPath" -}}
/etc/abnchd/config.yaml
{{- end }}

{{/* The service Certificate and the Secret holding its key pair. */}}
{{- define "abnchd.certificateName" -}}
{{- printf "%s-tls" (include "abnchd.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "abnchd.tlsSecretName" -}}
{{- include "abnchd.certificateName" . }}
{{- end }}
