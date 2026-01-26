import type { IObjectMeta } from "@kubernetes-models/apimachinery/apis/meta/v1/ObjectMeta"
import { Model as BaseModel } from "@kubernetes-models/base"

type Condition = Array<{
  type: string
  status: string
  lastTransitionTime: string
  message?: string
  reason: string
}>
type Status = {
  conditions?: Condition
  [key: string]: any
}

// XRD Model Registry types
export interface XrdModelRegistryEntry {
  modelClass: typeof Model
  group: string
  kind: string
}

export interface XrdModelRegistry {
  [key: string]: XrdModelRegistryEntry
}

// Global registry for XRD models
const xrdModelRegistry: XrdModelRegistry = {}

/**
 * Register an XRD model class with the global registry
 * @param group - The API group (e.g., "workspace.fabrique.social.gouv.fr")
 * @param kind - The resource kind (e.g., "XRedis")
 * @returns Class decorator function
 */
export function registerXrdModel(group: string, kind: string) {
  return function <T extends new (...args: any[]) => Model<any>>(target: T): T {
    const registryKey = `${group}/${kind}`

    xrdModelRegistry[registryKey] = {
      modelClass: target as any,
      group,
      kind,
    }

    return target
  }
}

/**
 * Get a registered XRD model class by group and kind
 * @param group - The API group
 * @param kind - The resource kind
 * @returns The registered model class or undefined if not found
 */
export function getRegisteredXrdModel(group: string, kind: string): typeof Model | undefined {
  const registryKey = `${group}/${kind}`
  return xrdModelRegistry[registryKey]?.modelClass
}

/**
 * Get a registered XRD model class by apiVersion and kind
 * @param apiVersion - The full API version (e.g., "workspace.fabrique.social.gouv.fr/v1alpha1")
 * @param kind - The resource kind
 * @returns The registered model class or undefined if not found
 */
export function getRegisteredXrdModelByApiVersion(
  apiVersion: string,
  kind: string
): typeof Model | undefined {
  // Extract group from apiVersion
  // If no '/' is present, use "core" as convention for core resources
  const group = apiVersion.includes("/") ? apiVersion.split("/")[0] : "core"
  return getRegisteredXrdModel(group, kind)
}

/**
 * Get all registered XRD models
 * @returns The complete registry
 */
export function getXrdModelRegistry(): XrdModelRegistry {
  return { ...xrdModelRegistry }
}

export class Model<T> extends BaseModel<T> {
  getMetadata(): IObjectMeta {
    const self = this as any
    if (!self.metadata) {
      throw new Error("No metadata found")
    }
    return self.metadata
  }

  getStatus(): Status {
    const self = this as any
    return self.status || {}
  }

  getKind(): string {
    const self = this as any
    return self.kind
  }

  getApiVersion(): string {
    const self = this as any
    return self.apiVersion
  }

  /**
   * Check if this resource is ready based on its status conditions
   * @returns true if the resource is ready, false otherwise
   */
  isReady(): boolean {
    try {
      // ProviderConfigs don't have status conditions, we assume they're always ready
      if ((this as any).kind === "ProviderConfig") {
        return true
      }

      // Check for Ready condition in status.conditions
      const conditions = this.getStatus()?.conditions
      if (!conditions || !Array.isArray(conditions)) {
        return false
      }

      const readyCondition = conditions.find((condition: any) => condition.type === "Ready")
      return readyCondition?.status === "True"
    } catch (_error) {
      return false
    }
  }

  /**
   * Get a specific condition from the resource status
   * @param conditionType - The type of condition to find
   * @returns The condition object or undefined if not found
   */
  getCondition(
    conditionType: string
  ):
    | { type: string; status: string; lastTransitionTime: string; message?: string; reason: string }
    | undefined {
    return this.getStatus()?.conditions?.find(condition => condition.type === conditionType)
  }

  /**
   * Check if a specific condition is true
   * @param conditionType - The type of condition to check
   * @returns true if the condition exists and its status is "True"
   */
  hasCondition(conditionType: string): boolean {
    const condition = this.getCondition(conditionType)
    return condition?.status === "True"
  }

  /**
   * Get the resource name
   * @returns The resource name
   */
  getName(): string | undefined {
    return this.getMetadata().name
  }

  /**
   * Get the resource namespace
   * @returns The resource namespace or undefined if not set
   */
  getNamespace(): string | undefined {
    return this.getMetadata().namespace
  }

  /**
   * Get an annotation value
   * @param key - The annotation key
   * @returns The annotation value or undefined if not found
   */
  getAnnotation(key: string): string | undefined {
    return this.getMetadata().annotations?.[key]
  }

  /**
   * Get a label value
   * @param key - The label key
   * @returns The label value or undefined if not found
   */
  getLabel(key: string): string | undefined {
    return this.getMetadata().labels?.[key]
  }

  /**
   * Check if the resource is paused
   * @returns true if the resource has the pause annotation set to "true"
   */
  isPaused(): boolean {
    return this.getAnnotation("crossplane.io/paused") === "true"
  }

  /**
   * Create a Usage resource to establish dependency relationships between resources
   * @param byResource - Optional resource that uses this resource
   * @returns Usage resource object
   */
  makeUsage(byResource?: Model<any>): any {
    const usageName = byResource
      ? `${byResource.getMetadata().name}-uses-${this.getMetadata().name}`
      : `protect-${this.getMetadata().name}`

    const usage: any = {
      apiVersion: "apiextensions.crossplane.io/v1alpha1",
      kind: "Usage",
      getMetadata() {
        return usageName
      },
      spec: {
        replayDeletion: true,
        of: {
          apiVersion: (this as any).apiVersion,
          kind: (this as any).kind,
          resourceRef: { name: this.getMetadata().name },
        },
      },
    }

    if (byResource) {
      usage.spec.by = {
        apiVersion: (byResource as any).apiVersion,
        kind: (byResource as any).kind,
        resourceRef: { name: byResource.getMetadata().name },
      }
    }

    return usage
  }
}
