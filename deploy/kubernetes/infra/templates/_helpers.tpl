{{/*
Pod and container security contexts for Pod Security "restricted".
Takes a workload's `security` values: user (the image's uid), fsGroup (its gid and the
volumes' group, default: user), readOnly (read-only root filesystem).
*/}}
{{- define "dsn.podSecurityContext" -}}
runAsNonRoot: true
runAsUser: {{ .user }}
runAsGroup: {{ .fsGroup | default .user }}
fsGroup: {{ .fsGroup | default .user }}
seccompProfile:
  type: RuntimeDefault
{{- end }}

{{- define "dsn.securityContext" -}}
allowPrivilegeEscalation: false
readOnlyRootFilesystem: {{ .readOnly | default false }}
capabilities:
  drop: [ALL]
{{- end }}

{{/*
A NetworkPolicy letting the rules' peers into the pods labelled app=<name>.
Takes (dict "app" <name> "rules" <rules> "gateway" .Values.networkPolicy.gateway); a rule is
{from: [app labels], gateway: the Gateway's proxy pods, namespace: any pod here, ports: [...]}.
No rules = nothing gets in.
*/}}
{{- define "dsn.networkPolicy" -}}
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ .app }}
spec:
  podSelector:
    matchLabels:
      app: {{ .app }}
  policyTypes: [Ingress]
  {{- with .rules }}
  ingress:
    {{- range . }}
    - from:
        {{- range .from }}
        - podSelector:
            matchLabels:
              app: {{ . }}
        {{- end }}
        {{- if .gateway }}
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: {{ $.gateway.namespace }}
          podSelector:
            matchLabels:
              {{- toYaml $.gateway.podLabels | nindent 14 }}
        {{- end }}
        {{- if .namespace }}
        - podSelector: {}
        {{- end }}
      {{- with .ports }}
      ports:
        {{- range . }}
        - port: {{ . }}
        {{- end }}
      {{- end }}
    {{- end }}
  {{- else }}
  ingress: []
  {{- end }}
{{- end }}
