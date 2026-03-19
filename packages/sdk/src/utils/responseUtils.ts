import {
  CrossplaneResourceEntry,
  ExtraResourceRequirement,
  KubernetesResource,
  KubernetesResourceLike,
} from "../types.ts"

import { areFieldRefsBroken } from "./FieldRef.ts"

interface CrossplaneFunctionComposite<T extends KubernetesResourceLike = KubernetesResourceLike> {
  resource: T
  connectionDetails?: Record<string, string>
}

interface CrossplaneFunctionDesiredResource<
  T extends KubernetesResourceLike = KubernetesResourceLike,
> {
  resources: Record<string, CrossplaneResourceEntry>
  composite: CrossplaneFunctionComposite<T>
  extraResourceRequirements?: Record<string, ExtraResourceRequirement>
}

export class CrossplaneFunctionResponse<T extends KubernetesResourceLike = KubernetesResourceLike> {
  resources: Record<string, CrossplaneResourceEntry>
  composite: CrossplaneFunctionComposite<T>
  extraResourceRequirements: Record<string, ExtraResourceRequirement> | undefined
  extraResources: Record<string, KubernetesResource[]> | undefined

  constructor(
    desired: CrossplaneFunctionDesiredResource<T>,
    extraResources?: Record<string, KubernetesResource[]>
  ) {
    this.resources = desired.resources
    this.composite = desired.composite
    this.extraResourceRequirements = desired.extraResourceRequirements
    this.extraResources = extraResources
  }

  updateResource(name: string, resource: CrossplaneResourceEntry): void {
    let compositeAsResource: KubernetesResource
    if ("toJSON" in resource.resource && typeof resource.resource.toJSON === "function") {
      compositeAsResource = resource.resource.toJSON()
    } else {
      compositeAsResource = resource.resource as KubernetesResource
    }
    const fallenBack = areFieldRefsBroken(compositeAsResource)
    if (fallenBack) {
      compositeAsResource.metadata.annotations = {
        ...compositeAsResource.metadata.annotations,
        "crossplane.io/paused": "true",
      }
    }
    this.resources[name] = resource
  }

  requestExtraResource<T extends KubernetesResource | KubernetesResourceLike = KubernetesResource>(
    name: string,
    requirement: ExtraResourceRequirement
  ): T[] | undefined {
    if (!this.extraResourceRequirements) {
      this.extraResourceRequirements = {}
    }
    this.extraResourceRequirements[name] = requirement

    if (this.extraResources) {
      return (this.extraResources[name] as T[]) || undefined
    }
  }
}

interface observedResourceError {
  resourceName: string
  resourceKind: string
  reason?: string
  message?: string
}

interface KubernetesCondition {
  type: string
  status: string
  reason?: string
  message?: string
}

export function getErrorsFromObservedResources(
  observedResources: Record<string, { resource: KubernetesResource }>
): observedResourceError[] {
  const errors: observedResourceError[] = []

  for (const resourceRef in observedResources) {
    const observedResource = observedResources[resourceRef]
    const resourceKind = observedResource.resource.kind
    const resourceName = observedResource.resource.metadata.name

    for (const condition of (observedResource.resource.status
      ?.conditions as KubernetesCondition[]) || []) {
      let errored = false
      if (condition.type === "Synced" && condition.status === "False") {
        errors.push({
          resourceName,
          resourceKind,
          reason: condition.reason,
          message: condition.message,
        })
        errored = true
      }
      if (condition.type === "Ready" && condition.status === "False") {
        errors.push({
          resourceName,
          resourceKind,
          reason: condition.reason,
          message: condition.message,
        })
        errored = true
      }
      if (
        errored &&
        observedResource.resource.apiVersion.includes("kubernetes.crossplane.io") &&
        observedResource.resource.kind === "Object"
      ) {
        const kubernetesRessource = observedResource.resource.status?.atProvider as {
          manifest: KubernetesResource
        }
        errors.push({
          resourceName: kubernetesRessource.manifest.metadata.name,
          resourceKind: kubernetesRessource.manifest.kind,
          reason: "KubernetesObjectError",
          message: JSON.stringify(kubernetesRessource.manifest?.status?.conditions || [], null, 2),
        })
      }
    }
  }
  return errors
}

export function buildResponse<T extends KubernetesResourceLike = KubernetesResourceLike>(
  composite: T,
  extraResources?: Record<string, KubernetesResource[]>,
  observedResources?: Record<string, { resource: KubernetesResource }>
): CrossplaneFunctionResponse<T> {
  let compositeAsResource: KubernetesResource
  if ("toJSON" in composite && typeof composite.toJSON === "function") {
    compositeAsResource = composite.toJSON()
  } else {
    compositeAsResource = composite as KubernetesResource
  }

  const errors = getErrorsFromObservedResources(observedResources || {})
  if (errors.length > 0) {
    compositeAsResource.status = {
      ...compositeAsResource.status,
      errors: errors,
    }
  }

  const desired: CrossplaneFunctionDesiredResource<T> = {
    resources: {},
    composite: {
      resource: compositeAsResource as T,
      connectionDetails: {},
    },
    extraResourceRequirements: {},
  }

  return new CrossplaneFunctionResponse<T>(desired, extraResources)
}
