import { Model } from "../Model/index.ts"
import {
  CompositeResourceEntry,
  CrossplaneDesiredResources,
  CrossplaneResourceEntry,
  ExtraResourceRequirement,
  KubernetesMetadata,
  KubernetesResource,
} from "../types.ts"

export class CrossplaneFunctionResponse {
  resources: Record<string, CrossplaneResourceEntry>
  composite: CompositeResourceEntry | undefined
  extraResourceRequirements: Record<string, ExtraResourceRequirement> | undefined
  extraResources: Record<string, KubernetesResource[]> | undefined

  constructor(
    desired: CrossplaneDesiredResources,
    extraResources?: Record<string, KubernetesResource[]>
  ) {
    this.resources = desired.resources
    this.composite = desired.composite
    this.extraResourceRequirements = desired.extraResourceRequirements
    this.extraResources = extraResources
  }

  updateResource(name: string, resource: CrossplaneResourceEntry): void {
    this.resources[name] = resource
  }

  requestExtraResource(
    name: string,
    requirement: ExtraResourceRequirement
  ): KubernetesResource[] | undefined {
    if (!this.extraResourceRequirements) {
      this.extraResourceRequirements = {}
    }
    this.extraResourceRequirements[name] = requirement

    if (this.extraResources) {
      return this.extraResources[name] || undefined
    }
  }

  toDesiredResources(): CrossplaneDesiredResources {
    return {
      resources: this.resources,
      composite: this.composite,
      extraResourceRequirements: this.extraResourceRequirements,
    }
  }
}

export function buildResponse(
  composite: Model<KubernetesResource>,
  extraResources?: Record<string, KubernetesResource[]>
): CrossplaneFunctionResponse {
  const desired: CrossplaneDesiredResources = {
    resources: {},
    composite: {
      resource: {
        apiVersion: composite.getApiVersion(),
        kind: composite.getKind(),
        metadata: {
          ...composite.getMetadata(),
          managedFields: undefined,
        } as KubernetesMetadata,
        status: {
          ...composite.getStatus(),
        },
      },
      connectionDetails: {},
    },
    extraResourceRequirements: {},
  }

  return new CrossplaneFunctionResponse(desired, extraResources)
}
