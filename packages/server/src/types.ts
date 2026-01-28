/**
 * Kubernetes resource metadata
 */
export interface KubernetesMetadata {
  name: string
  namespace?: string
  labels?: Record<string, string>
  annotations?: Record<string, string>
  [key: string]: unknown
}

/**
 * Base Kubernetes resource
 */
export interface KubernetesResource {
  apiVersion: string
  kind: string
  metadata: KubernetesMetadata
  spec?: Record<string, unknown>
  status?: Record<string, unknown>
  [key: string]: unknown
}

/**
 * Extra resource requirement (for additional resources needed by the function)
 */
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

/**
 * Desired composite resource entry returned by a composition function
 */
export interface CompositeResourceEntry {
  /**
   * Full composite resource object (apiVersion, kind, metadata, spec, status, ...)
   */
  resource: KubernetesResource

  /**
   * Connection details to expose on the composite resource
   */
  connectionDetails?: Record<string, string>
}

/**
 * Crossplane composite resource (observed)
 */
export interface CrossplaneCompositeResource {
  resource: KubernetesResource
  status?: Record<string, unknown>
}

/**
 * Crossplane observed resources
 */
export interface CrossplaneObservedResources {
  composite: CrossplaneCompositeResource
  resources: Record<string, KubernetesResource>
}

/**
 * Crossplane resource entry (desired composed resources)
 */
export interface CrossplaneResourceEntry {
  resource: KubernetesResource
  ready?: boolean
  connectionDetails?: string[]
}

/**
 * Crossplane desired resources
 */
export interface CrossplaneDesiredResources {
  /**
   * Desired composed resources
   */
  resources: Record<string, CrossplaneResourceEntry>

  /**
   * Desired state of the composite resource itself (spec, status, metadata, connection details)
   */
  composite?: CompositeResourceEntry

  /**
   * Extra resource requirements requested by the function
   */
  extraResourceRequirements?: Record<string, ExtraResourceRequirement>
}

/**
 * Crossplane input structure
 */
export interface CrossplaneInput {
  observed: CrossplaneObservedResources
  desired?: CrossplaneDesiredResources
}

/**
 * Log entry structure
 */
export interface LogEntry {
  level: string
  message: string
  timestamp?: string
  [key: string]: unknown
}

/**
 * Function result type
 */
export type FunctionResult = CrossplaneDesiredResources | Record<string, unknown>

/**
 * Function input type
 */
export type FunctionInput = CrossplaneInput | Record<string, unknown>

/**
 * Root object passed by the Node.js executor to user composition functions.
 *
 * This represents the runtime request shape when running under Crossplane:
 * - `composite` is typically a domain model instance created by createModel.
 * - `extraResources` contains any resources injected based on
 *   extraResourceRequirements from a previous run.
 */
export interface RunFunctionRequest<
  TComposite = unknown,
  TObservedResources = Record<string, unknown>,
  TExtraResources = Record<string, unknown[]>,
  TContext = Record<string, unknown>,
> {
  composite: TComposite
  observed: TObservedResources
  extraResources?: TExtraResources
  context: TContext
}

/**
 * Type for composition functions
 */
export type CompositionFunction = (
  input: CrossplaneInput
) => CrossplaneDesiredResources | Promise<CrossplaneDesiredResources>

/**
 * Response from running code
 */
export interface NodeResponse {
  /**
   * The result of running the code, if successful
   */
  result?: FunctionResult

  /**
   * Error information if the code execution failed
   */
  error?: NodeError

  /**
   * Captured console logs from the code execution
   */
  logs?: Array<LogEntry>
}

/**
 * Error information
 */
export interface NodeError {
  /**
   * Error code
   */
  code: number

  /**
   * Error message
   */
  message: string

  /**
   * Stack trace if available
   */
  stack?: string
}

/**
 * Request to run code
 */
export interface NodeRequest {
  /**
   * The code to run
   */
  code: string

  /**
   * Dependencies to install before running the code
   */
  dependencies?: Record<string, string>

  /**
   * The input data for the code
   */
  input: FunctionInput
}
