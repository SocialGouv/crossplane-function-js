/**
 * Type definitions for @crossplane-js/sdk
 */

// Kubernetes resource metadata
export interface KubernetesMetadata {
  name: string
  namespace?: string
  labels?: Record<string, string>
  annotations?: Record<string, string>
  [key: string]: unknown
}

// Base Kubernetes resource
export interface KubernetesResource {
  apiVersion: string
  kind: string
  metadata: KubernetesMetadata
  spec?: Record<string, unknown>
  status?: Record<string, unknown>
  [key: string]: unknown
}

// Union type for resources that can be serialized to KubernetesResource
export type KubernetesResourceLike = KubernetesResource | { toJSON(): KubernetesResource }

// Extra resource requirement (for additional resources needed by the function)
export interface ExtraResourceRequirement {
  apiVersion: string
  kind: string
  matchLabels?: Record<string, string>
  matchName?: string
  /**
   * Optional namespace to constrain the selector.
   * - Omit to match cluster-scoped resources, or to match namespaced
   *   resources by labels across all namespaces.
   */
  namespace?: string
}

// Desired composite resource entry returned by a composition function
export interface CompositeResourceEntry {
  resource: KubernetesResourceLike
  // Connection details to expose on the composite resource
  connectionDetails?: Record<string, string>
}

// Crossplane composite resource
export interface CrossplaneCompositeResource {
  resource: KubernetesResource
  status?: Record<string, unknown>
}

// Crossplane observed resources
export interface CrossplaneObservedResources {
  composite: CrossplaneCompositeResource
  resources: Record<string, KubernetesResource>
}

// Crossplane resource entry
export interface CrossplaneResourceEntry {
  resource: KubernetesResourceLike
  ready?: boolean
  connectionDetails?: string[]
}

// Crossplane desired resources
export interface CrossplaneDesiredResources {
  // Desired composed resources
  resources: Record<string, CrossplaneResourceEntry>
  // Desired state of the composite resource itself (spec, status, metadata, connection details)
  composite?: CompositeResourceEntry
  // Extra resource requirements requested by the function
  extraResourceRequirements?: Record<string, ExtraResourceRequirement>
}

// Crossplane input structure
export interface CrossplaneInput {
  observed: CrossplaneObservedResources
  desired?: CrossplaneDesiredResources
}

// Function result type
export type FunctionResult = CrossplaneDesiredResources | Record<string, unknown>

// Function input type
export type FunctionInput = CrossplaneInput | Record<string, unknown>

// Root object passed to composition functions when running under the
// Crossplane Node.js executor. This mirrors the server-side
// RunFunctionRequest but is generic so domain SDKs can type it nicely.
export interface RunFunctionRequest<
  TComposite = KubernetesResourceLike,
  TExtraResources = Record<string, unknown[]>
> {
  composite: TComposite
  extraResources?: TExtraResources
}

// Type for composition functions
export type CompositionFunction = (
  input: CrossplaneInput
) => CrossplaneDesiredResources | Promise<CrossplaneDesiredResources>
