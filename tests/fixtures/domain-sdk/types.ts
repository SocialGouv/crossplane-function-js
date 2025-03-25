/**
 * Re-export types from the main project for use in domain-sdk functions
 */

// Kubernetes resource metadata
export interface KubernetesMetadata {
  name: string;
  namespace?: string;
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
  [key: string]: unknown;
}

// Base Kubernetes resource
export interface KubernetesResource {
  apiVersion: string;
  kind: string;
  metadata: KubernetesMetadata;
  spec?: Record<string, unknown>;
  status?: Record<string, unknown>;
  [key: string]: unknown;
}

// Crossplane composite resource
export interface CrossplaneCompositeResource {
  resource: KubernetesResource;
  status?: Record<string, unknown>;
}

// Crossplane observed resources
export interface CrossplaneObservedResources {
  composite: CrossplaneCompositeResource;
  resources: Record<string, KubernetesResource>;
}

// Crossplane resource entry
export interface CrossplaneResourceEntry {
  resource: KubernetesResource;
  ready?: boolean;
  connectionDetails?: string[];
}

// Crossplane desired resources
export interface CrossplaneDesiredResources {
  resources: Record<string, CrossplaneResourceEntry>;
}

// Crossplane input structure
export interface CrossplaneInput {
  observed: CrossplaneObservedResources;
  desired?: CrossplaneDesiredResources;
}

// Function result type
export type FunctionResult = CrossplaneDesiredResources | Record<string, unknown>;

// Function input type
export type FunctionInput = CrossplaneInput | Record<string, unknown>;

// Type for composition functions
export type CompositionFunction = (input: CrossplaneInput) => CrossplaneDesiredResources | Promise<CrossplaneDesiredResources>;