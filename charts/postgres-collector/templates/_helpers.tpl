{{/*
Expand the name of the chart.
*/}}
{{- define "postgres-collector.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "postgres-collector.fullname" -}}
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
{{- define "postgres-collector.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "postgres-collector.labels" -}}
helm.sh/chart: {{ include "postgres-collector.chart" . }}
{{ include "postgres-collector.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "postgres-collector.selectorLabels" -}}
app.kubernetes.io/name: {{ include "postgres-collector.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "postgres-collector.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "postgres-collector.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
PostgreSQL password secret
*/}}
{{- define "postgres-collector.postgresqlSecretName" -}}
{{- if .Values.postgresql.existingSecret }}
{{- .Values.postgresql.existingSecret }}
{{- else }}
{{- include "postgres-collector.fullname" . }}-postgresql
{{- end }}
{{- end }}

{{/*
New Relic license secret
*/}}
{{- define "postgres-collector.newrelicSecretName" -}}
{{- if .Values.newrelic.existingSecret }}
{{- .Values.newrelic.existingSecret }}
{{- else }}
{{- include "postgres-collector.fullname" . }}-newrelic
{{- end }}
{{- end }}

{{/*
PgBouncer password secret
*/}}
{{- define "postgres-collector.pgbouncerSecretName" -}}
{{- if .Values.pgbouncer.existingSecret }}
{{- .Values.pgbouncer.existingSecret }}
{{- else }}
{{- include "postgres-collector.fullname" . }}-pgbouncer
{{- end }}
{{- end }}