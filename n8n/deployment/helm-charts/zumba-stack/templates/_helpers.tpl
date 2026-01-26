{{/*
Expand the name of the chart.
*/}}
{{- define "n8n-stack.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "n8n-stack.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "n8n-stack.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "n8n-stack.labels" -}}
helm.sh/chart: {{ include "n8n-stack.chart" . }}
{{ include "n8n-stack.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "n8n-stack.selectorLabels" -}}
app.kubernetes.io/name: {{ include "n8n-stack.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
n8n specific labels
*/}}
{{- define "n8n-stack.n8n.labels" -}}
{{ include "n8n-stack.labels" . }}
app.kubernetes.io/component: n8n
{{- end }}

{{/*
n8n selector labels
*/}}
{{- define "n8n-stack.n8n.selectorLabels" -}}
{{ include "n8n-stack.selectorLabels" . }}
app.kubernetes.io/component: n8n
{{- end }}

{{/*
postgres specific labels
*/}}
{{- define "n8n-stack.postgres.labels" -}}
{{ include "n8n-stack.labels" . }}
app.kubernetes.io/component: postgres
{{- end }}

{{/*
postgres selector labels
*/}}
{{- define "n8n-stack.postgres.selectorLabels" -}}
{{ include "n8n-stack.selectorLabels" . }}
app.kubernetes.io/component: postgres
{{- end }}
