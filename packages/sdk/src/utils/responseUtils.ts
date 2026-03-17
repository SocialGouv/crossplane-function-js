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
    let kubernetesResource: KubernetesResource
    if ("toJSON" in resource.resource && typeof resource.resource.toJSON === "function") {
      kubernetesResource = resource.resource.toJSON()
    } else {
      kubernetesResource = resource.resource as KubernetesResource
    }
    const fallenBack = areFieldRefsBroken(kubernetesResource)
    if (fallenBack) {
      kubernetesResource.metadata.annotations = {
        ...kubernetesResource.metadata.annotations,
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

export function buildResponse<T extends KubernetesResourceLike = KubernetesResourceLike>(
  composite: T,
  extraResources?: Record<string, KubernetesResource[]>
): CrossplaneFunctionResponse<T> {
  const desired: CrossplaneFunctionDesiredResource<T> = {
    resources: {},
    composite: {
      resource: composite,
      connectionDetails: {},
    },
    extraResourceRequirements: {},
  }

  return new CrossplaneFunctionResponse<T>(desired, extraResources)
}
