import type { KubernetesResource, KubernetesResourceLike } from '../types.ts'

/**
 * Normalize a KubernetesResourceLike into a plain KubernetesResource.
 * If the object implements toJSON(), that result is used.
 */
function normalizeResource(resource: KubernetesResourceLike): KubernetesResource {
  if (typeof (resource as any)?.toJSON === 'function') {
    return (resource as any).toJSON() as KubernetesResource
  }
  return resource as KubernetesResource
}

/**
 * Create a desired-safe composite resource object from an observed resource or model.
 *
 * We currently only care about driving the composite **status** (and
 * identifying which XR it is), so we:
 * - normalize models (using toJSON()) to plain Kubernetes resources
 * - construct a minimal object containing:
 *   - apiVersion
 *   - kind
 *   - metadata.name / metadata.namespace (for identity)
 *   - status (as provided by the normalized object)
 * - intentionally drop spec and all other metadata fields so JS cannot
 *   accidentally try to own spec or server-managed metadata.
 */
export function toDesiredCompositeResource(
  resource: KubernetesResourceLike
): KubernetesResource {
  const normalized = normalizeResource(resource)

  return {
    apiVersion: normalized.apiVersion,
    kind: normalized.kind,
    metadata: {
      name: normalized.metadata?.name,
      namespace: normalized.metadata?.namespace,
    },
    status: normalized.status,
  }
}
