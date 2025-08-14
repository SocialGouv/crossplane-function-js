import * as kubernetesModels from "kubernetes-models"

import { withFieldRefsClassFactory } from "../utils/FieldRef"

// Core v1 resources with FieldRef support
export const v1 = {
  // Core resources
  ConfigMap: withFieldRefsClassFactory(kubernetesModels.v1.ConfigMap),
  Secret: withFieldRefsClassFactory(kubernetesModels.v1.Secret),
  Service: withFieldRefsClassFactory(kubernetesModels.v1.Service),
  Pod: withFieldRefsClassFactory(kubernetesModels.v1.Pod),
  Namespace: withFieldRefsClassFactory(kubernetesModels.v1.Namespace),

  // Storage resources
  PersistentVolume: withFieldRefsClassFactory(kubernetesModels.v1.PersistentVolume),
  PersistentVolumeClaim: withFieldRefsClassFactory(kubernetesModels.v1.PersistentVolumeClaim),

  // Service account and RBAC
  ServiceAccount: withFieldRefsClassFactory(kubernetesModels.v1.ServiceAccount),

  // Events and endpoints
  Event: withFieldRefsClassFactory(kubernetesModels.v1.Event),
  Endpoints: withFieldRefsClassFactory(kubernetesModels.v1.Endpoints),

  // Resource management
  ResourceQuota: withFieldRefsClassFactory(kubernetesModels.v1.ResourceQuota),
  LimitRange: withFieldRefsClassFactory(kubernetesModels.v1.LimitRange),

  // Node resources
  Node: withFieldRefsClassFactory(kubernetesModels.v1.Node),

  // Controllers
  ReplicationController: withFieldRefsClassFactory(kubernetesModels.v1.ReplicationController),

  // Pod templates
  PodTemplate: withFieldRefsClassFactory(kubernetesModels.v1.PodTemplate),

  // Bindings
  Binding: withFieldRefsClassFactory(kubernetesModels.v1.Binding),

  // Component status
  ComponentStatus: withFieldRefsClassFactory(kubernetesModels.v1.ComponentStatus),
}

// Apps resources with FieldRef support
export const apps = {
  v1: {
    Deployment: withFieldRefsClassFactory(kubernetesModels.apps.v1.Deployment),
    StatefulSet: withFieldRefsClassFactory(kubernetesModels.apps.v1.StatefulSet),
    DaemonSet: withFieldRefsClassFactory(kubernetesModels.apps.v1.DaemonSet),
    ReplicaSet: withFieldRefsClassFactory(kubernetesModels.apps.v1.ReplicaSet),
  },
}

// Batch resources with FieldRef support
export const batch = {
  v1: {
    Job: withFieldRefsClassFactory(kubernetesModels.batch.v1.Job),
    CronJob: withFieldRefsClassFactory(kubernetesModels.batch.v1.CronJob),
  },
}

// Networking resources with FieldRef support
export const networking = {
  v1: {
    Ingress: withFieldRefsClassFactory(kubernetesModels.networkingK8sIo.v1.Ingress),
    NetworkPolicy: withFieldRefsClassFactory(kubernetesModels.networkingK8sIo.v1.NetworkPolicy),
    IngressClass: withFieldRefsClassFactory(kubernetesModels.networkingK8sIo.v1.IngressClass),
  },
}

// RBAC resources with FieldRef support
export const rbac = {
  v1: {
    Role: withFieldRefsClassFactory(kubernetesModels.rbacAuthorizationK8sIo.v1.Role),
    RoleBinding: withFieldRefsClassFactory(kubernetesModels.rbacAuthorizationK8sIo.v1.RoleBinding),
    ClusterRole: withFieldRefsClassFactory(kubernetesModels.rbacAuthorizationK8sIo.v1.ClusterRole),
    ClusterRoleBinding: withFieldRefsClassFactory(
      kubernetesModels.rbacAuthorizationK8sIo.v1.ClusterRoleBinding
    ),
  },
}

// Storage resources with FieldRef support
export const storage = {
  v1: {
    StorageClass: withFieldRefsClassFactory(kubernetesModels.storageK8sIo.v1.StorageClass),
    VolumeAttachment: withFieldRefsClassFactory(kubernetesModels.storageK8sIo.v1.VolumeAttachment),
    CSIDriver: withFieldRefsClassFactory(kubernetesModels.storageK8sIo.v1.CSIDriver),
    CSINode: withFieldRefsClassFactory(kubernetesModels.storageK8sIo.v1.CSINode),
    CSIStorageCapacity: withFieldRefsClassFactory(
      kubernetesModels.storageK8sIo.v1.CSIStorageCapacity
    ),
  },
}

// Autoscaling resources with FieldRef support
export const autoscaling = {
  v1: {
    HorizontalPodAutoscaler: withFieldRefsClassFactory(
      kubernetesModels.autoscaling.v1.HorizontalPodAutoscaler
    ),
  },
  v2: {
    HorizontalPodAutoscaler: withFieldRefsClassFactory(
      kubernetesModels.autoscaling.v2.HorizontalPodAutoscaler
    ),
  },
}

// Policy resources with FieldRef support
export const policy = {
  v1: {
    PodDisruptionBudget: withFieldRefsClassFactory(kubernetesModels.policy.v1.PodDisruptionBudget),
  },
}

// API extensions resources with FieldRef support
export const apiextensions = {
  v1: {
    CustomResourceDefinition: withFieldRefsClassFactory(
      kubernetesModels.apiextensionsK8sIo.v1.CustomResourceDefinition
    ),
  },
}

// Certificates resources with FieldRef support
export const certificates = {
  v1: {
    CertificateSigningRequest: withFieldRefsClassFactory(
      kubernetesModels.certificatesK8sIo.v1.CertificateSigningRequest
    ),
  },
}

// Coordination resources with FieldRef support
export const coordination = {
  v1: {
    Lease: withFieldRefsClassFactory(kubernetesModels.coordinationK8sIo.v1.Lease),
  },
}

// Events resources with FieldRef support
export const events = {
  v1: {
    Event: withFieldRefsClassFactory(kubernetesModels.eventsK8sIo.v1.Event),
  },
}

// Also export the original kubernetes-models for reference if needed
export { kubernetesModels }

// Export types for convenience
export type {
  IConfigMap,
  IPod,
  IService,
  ISecret,
  INamespace,
  IPersistentVolume,
  IPersistentVolumeClaim,
  IServiceAccount,
} from "kubernetes-models/v1"

export type { IDeployment, IStatefulSet, IDaemonSet, IReplicaSet } from "kubernetes-models/apps/v1"

export type { IJob, ICronJob } from "kubernetes-models/batch/v1"
