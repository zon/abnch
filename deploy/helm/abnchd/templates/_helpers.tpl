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
