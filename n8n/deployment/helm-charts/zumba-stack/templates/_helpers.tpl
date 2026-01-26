{{/*
Expand the name of the chart.
*/}}
{{- define "zumba-stack.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "zumba-stack.fullname" -}}
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
{{- define "zumba-stack.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "zumba-stack.labels" -}}
helm.sh/chart: {{ include "zumba-stack.chart" . }}
{{ include "zumba-stack.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "zumba-stack.selectorLabels" -}}
app.kubernetes.io/name: {{ include "zumba-stack.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
n8n specific labels
*/}}
{{- define "zumba-stack.n8n.labels" -}}
{{ include "zumba-stack.labels" . }}
app.kubernetes.io/component: n8n
{{- end }}

{{/*
n8n selector labels
*/}}
{{- define "zumba-stack.n8n.selectorLabels" -}}
{{ include "zumba-stack.selectorLabels" . }}
app.kubernetes.io/component: n8n
{{- end }}

{{/*
postgres specific labels
*/}}
{{- define "zumba-stack.postgres.labels" -}}
{{ include "zumba-stack.labels" . }}
app.kubernetes.io/component: postgres
{{- end }}

{{/*
postgres selector labels
*/}}
{{- define "zumba-stack.postgres.selectorLabels" -}}
{{ include "zumba-stack.selectorLabels" . }}
app.kubernetes.io/component: postgres
{{- end }}
