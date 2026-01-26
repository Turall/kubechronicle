{{/*
Expand the name of the chart.
*/}}
{{- define "kubechronicle.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "kubechronicle.fullname" -}}
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
{{- define "kubechronicle.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "kubechronicle.labels" -}}
helm.sh/chart: {{ include "kubechronicle.chart" . }}
{{ include "kubechronicle.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kubechronicle.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kubechronicle.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Component labels
*/}}
{{- define "kubechronicle.componentLabels" -}}
{{- $context := index . 0 -}}
{{- $component := index . 1 -}}
{{ include "kubechronicle.labels" $context }}
app.kubernetes.io/component: {{ $component }}
{{- end }}

{{/*
Component selector labels
*/}}
{{- define "kubechronicle.componentSelectorLabels" -}}
{{- $context := index . 0 -}}
{{- $component := index . 1 -}}
{{ include "kubechronicle.selectorLabels" $context }}
app.kubernetes.io/component: {{ $component }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "kubechronicle.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (printf "%s-webhook" (include "kubechronicle.fullname" .)) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Get namespace
*/}}
{{- define "kubechronicle.namespace" -}}
{{- if .Values.namespace.create }}
{{- .Values.namespace.name }}
{{- else }}
{{- default .Release.Namespace .Values.namespace.name }}
{{- end }}
{{- end }}

{{/*
Get image registry
*/}}
{{- define "kubechronicle.imageRegistry" -}}
{{- if .Values.global.imageRegistry }}
{{- printf "%s/" .Values.global.imageRegistry }}
{{- else }}
{{- "" }}
{{- end }}
{{- end }}

{{/*
Get image pull secrets
*/}}
{{- define "kubechronicle.imagePullSecrets" -}}
{{- if .Values.global.imagePullSecrets }}
imagePullSecrets:
{{- range .Values.global.imagePullSecrets }}
  - name: {{ .name }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Get API service name
*/}}
{{- define "kubechronicle.apiServiceName" -}}
{{- printf "%s-api" (include "kubechronicle.fullname" .) }}
{{- end }}

{{/*
Get webhook service name
*/}}
{{- define "kubechronicle.webhookServiceName" -}}
{{- printf "%s-webhook" (include "kubechronicle.fullname" .) }}
{{- end }}

{{/*
Get webhook service FQDN
*/}}
{{- define "kubechronicle.webhookServiceFQDN" -}}
{{- printf "%s.%s.svc" (include "kubechronicle.webhookServiceName" .) (include "kubechronicle.namespace" .) }}
{{- end }}
