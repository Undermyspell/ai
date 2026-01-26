{{/*
Expand the name of the chart.
*/}}
{{- define "zumba.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "zumba.fullname" -}}
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
{{- define "zumba.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "zumba.labels" -}}
helm.sh/chart: {{ include "zumba.chart" . }}
{{ include "zumba.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "zumba.selectorLabels" -}}
app.kubernetes.io/name: {{ include "zumba.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
n8n specific labels
*/}}
{{- define "zumba.n8n.labels" -}}
{{ include "zumba.labels" . }}
app.kubernetes.io/component: n8n
{{- end }}

{{/*
n8n selector labels
*/}}
{{- define "zumba.n8n.selectorLabels" -}}
{{ include "zumba.selectorLabels" . }}
app.kubernetes.io/component: n8n
{{- end }}

{{/*
postgres specific labels
*/}}
{{- define "zumba.postgres.labels" -}}
{{ include "zumba.labels" . }}
app.kubernetes.io/component: postgres
{{- end }}

{{/*
postgres selector labels
*/}}
{{- define "zumba.postgres.selectorLabels" -}}
{{ include "zumba.selectorLabels" . }}
app.kubernetes.io/component: postgres
{{- end }}
