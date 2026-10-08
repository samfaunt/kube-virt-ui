{{- define "kvui.name" -}}kubevirt-ui{{- end -}}

{{- define "kvui.labels" -}}
app.kubernetes.io/name: kubevirt-ui
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end -}}

{{- define "kvui.selector" -}}
app.kubernetes.io/name: kubevirt-ui
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "kvui.secretName" -}}
{{- .Values.existingSecret | default (printf "%s-key" .Release.Name) -}}
{{- end -}}

{{/*
Tenant permission tiers. ClusterRoles named kubevirt-ui-<role> are bound per
namespace by the backend (RoleBinding -> per-user ServiceAccount), so these
rules only ever apply inside tenant namespaces. Deliberately narrower than
the built-in edit role: no pods, secrets, deployments or exec.
*/}}
{{- define "kvui.rules.viewer" -}}
- apiGroups: [kubevirt.io]
  resources: [virtualmachines, virtualmachineinstances, virtualmachineinstancemigrations]
  verbs: [get, list, watch]
- apiGroups: [cdi.kubevirt.io]
  resources: [datavolumes]
  verbs: [get, list, watch]
- apiGroups: [snapshot.kubevirt.io]
  resources: [virtualmachinesnapshots, virtualmachinerestores]
  verbs: [get, list, watch]
- apiGroups: [instancetype.kubevirt.io]
  resources: [virtualmachineinstancetypes, virtualmachinepreferences]
  verbs: [get, list, watch]
- apiGroups: [""]
  resources: [persistentvolumeclaims, events, resourcequotas, limitranges]
  verbs: [get, list, watch]
- apiGroups: [subresources.kubevirt.io]
  resources: [virtualmachineinstances/guestosinfo, virtualmachineinstances/filesystemlist, virtualmachineinstances/userlist]
  verbs: [get]
{{- end -}}

{{- define "kvui.rules.operator" -}}
- apiGroups: [subresources.kubevirt.io]
  resources: [virtualmachines/start, virtualmachines/stop, virtualmachines/restart,
              virtualmachineinstances/pause, virtualmachineinstances/unpause, virtualmachineinstances/softreboot]
  verbs: [update]
- apiGroups: [subresources.kubevirt.io]
  resources: [virtualmachineinstances/vnc, virtualmachineinstances/vnc/screenshot, virtualmachineinstances/console]
  verbs: [get]
{{- end -}}

{{- define "kvui.rules.owner" -}}
- apiGroups: [kubevirt.io]
  resources: [virtualmachines]
  verbs: [create, update, patch, delete]
- apiGroups: [kubevirt.io]
  resources: [virtualmachineinstances]
  verbs: [delete]
- apiGroups: [subresources.kubevirt.io]
  resources: [virtualmachines/addvolume, virtualmachines/removevolume,
              virtualmachineinstances/addvolume, virtualmachineinstances/removevolume]
  verbs: [update]
- apiGroups: [cdi.kubevirt.io]
  resources: [datavolumes]
  verbs: [create, update, patch, delete]
- apiGroups: [cdi.kubevirt.io]
  resources: [datavolumes/source]   # cloning a disk from another DataVolume
  verbs: [create]
- apiGroups: [upload.cdi.kubevirt.io]
  resources: [uploadtokenrequests]  # browser uploads of ISO/qcow2 images
  verbs: [create]
- apiGroups: [snapshot.kubevirt.io]
  resources: [virtualmachinesnapshots, virtualmachinerestores]
  verbs: [create, delete]
- apiGroups: [instancetype.kubevirt.io]
  resources: [virtualmachineinstancetypes, virtualmachinepreferences]
  verbs: [create, update, patch, delete]
- apiGroups: [""]
  resources: [persistentvolumeclaims]
  verbs: [create, update, patch, delete]
{{- end -}}
