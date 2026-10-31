{{- define "agentfence.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "agentfence.fullname" -}}
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

{{- define "agentfence.labels" -}}
helm.sh/chart: {{ include "agentfence.name" . }}-{{ .Chart.Version }}
{{ include "agentfence.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: agentfence
{{- end }}

{{- define "agentfence.selectorLabels" -}}
app.kubernetes.io/name: {{ include "agentfence.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "agentfence.proxyImage" -}}
{{- $registry := .Values.global.imageRegistry | default .Values.proxy.image.repository -}}
{{- printf "%s:%s" (include "agentfence.imageRepo" (dict "registry" $registry "repo" .Values.proxy.image.repository)) .Values.proxy.image.tag }}
{{- end }}

{{- define "agentfence.imageRepo" -}}
{{- if .registry }}{{ .registry }}/{{ end }}{{ .repo }}
{{- end }}
