{{/*
Expand the name of the chart.
*/}}
{{- define "database-intelligence.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "database-intelligence.fullname" -}}
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
{{- define "database-intelligence.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "database-intelligence.labels" -}}
helm.sh/chart: {{ include "database-intelligence.chart" . }}
{{ include "database-intelligence.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Values.commonLabels }}
{{ toYaml .Values.commonLabels }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "database-intelligence.selectorLabels" -}}
app.kubernetes.io/name: {{ include "database-intelligence.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "database-intelligence.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "database-intelligence.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Return the proper Database Intelligence image name
*/}}
{{- define "database-intelligence.image" -}}
{{- with .Values.image }}
{{- if .digest }}
{{- printf "%s/%s@%s" .registry .repository .digest }}
{{- else }}
{{- printf "%s/%s:%s" .registry .repository (.tag | default $.Chart.AppVersion) }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Return the proper Docker Image Registry Secret Names
*/}}
{{- define "database-intelligence.imagePullSecrets" -}}
{{- include "common.images.pullSecrets" (dict "images" (list .Values.image) "global" .Values.global) -}}
{{- end -}}

{{/*
Create the name of the config secret to use
*/}}
{{- define "database-intelligence.secretName" -}}
{{- printf "%s-secret" (include "database-intelligence.fullname" .) }}
{{- end }}

{{/*
Create the name of the config map to use
*/}}
{{- define "database-intelligence.configMapName" -}}
{{- printf "%s-config" (include "database-intelligence.fullname" .) }}
{{- end }}

{{/*
Return true if a secret should be created
*/}}
{{- define "database-intelligence.createSecret" -}}
{{- if or .Values.config.postgres.enabled .Values.config.mysql.enabled .Values.config.newrelic.licenseKey }}
    {{- true -}}
{{- end -}}
{{- end -}}

{{/*
Get the namespace to use
*/}}
{{- define "database-intelligence.namespace" -}}
{{- if .Values.namespaceOverride }}
{{- .Values.namespaceOverride }}
{{- else }}
{{- .Release.Namespace }}
{{- end }}
{{- end }}