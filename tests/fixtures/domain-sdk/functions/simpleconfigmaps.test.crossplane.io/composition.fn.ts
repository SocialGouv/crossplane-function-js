import {
  logger,
  FieldRef,
  v1,
  toDesiredCompositeResource,
} from '@crossplane-js/sdk'
import type {
  CrossplaneDesiredResources,
  RunFunctionRequest,
} from '@crossplane-js/sdk'

import type { SimpleConfigMap } from '../../models/test.crossplane.io/v1beta1'

type SimpleRunRequest = RunFunctionRequest<SimpleConfigMap>

export default function (
  { composite, extraResources }: SimpleRunRequest
): CrossplaneDesiredResources {
  logger.info('SimpleConfigMap composition function started')
  logger.info(`composite:${JSON.stringify(composite)}`)

  const namespace = composite.getNamespace()
  const isReady = composite.isReady()

  logger.info(`Namespace: ${namespace}, Ready: ${isReady}`)
  logger.info(composite)

  // Transform data to uppercase for testing (same as original test logic)
  const transformedData: Record<string, string> = {}
  Object.entries(composite.spec.data).forEach(([key, value]) => {
    transformedData[key.toUpperCase()] = String(value).toUpperCase()
  })

  const testConfigMap = new v1.ConfigMap({
    metadata: {
      name: 'generated-configmap',
      namespace: namespace || 'test-xfuncjs',
      labels: {
        // Validate FieldRef resolution end-to-end: this should become the XR name.
        // `composite` here is the XR model instance.
        'crossplane-js.dev/xr-name': new FieldRef(
          composite as any,
          '$.metadata.name',
          'MISSING-XR-NAME'
        ),
      },
      annotations: (() => {
        // Expose extraResource injection results in a deterministic way that the
        // bash E2E harness can assert on.
        //
        // Note: Crossplane may call the function at least twice:
        // 1) first without extraResources (we request them)
        // 2) then again with extraResources populated
        //
        // These annotations will converge to expected values once Crossplane
        // injects the requested extra resources.
        const extra = (extraResources || {}) as Record<string, any[]>

        const nsCMs = extra.nsConfigMap || []
        const allNsCMs = extra.allNsConfigMaps || []
        const nsObjs = extra.testNamespace || []

        const names = (items: any[]) =>
          items
            .map((i) => {
              const n = i?.metadata?.name
              const ns = i?.metadata?.namespace
              return ns ? `${ns}/${n}` : n
            })
            .filter(Boolean)
            .sort()
            .join(',')

        return {
          'crossplane-js.dev/e2e-extra-ns-cm-count': String(nsCMs.length),
          'crossplane-js.dev/e2e-extra-allns-cm-count': String(allNsCMs.length),
          'crossplane-js.dev/e2e-extra-namespace-count': String(nsObjs.length),

          // Useful for debugging failures in CI.
          'crossplane-js.dev/e2e-extra-ns-cm-names': names(nsCMs),
          'crossplane-js.dev/e2e-extra-allns-cm-names': names(allNsCMs),
          'crossplane-js.dev/e2e-extra-namespace-names': names(nsObjs),
        }
      })(),
    },
    data: transformedData,
  })
  
  // Example: mutate status to demonstrate that status is carried through
  composite.status = {
    foo: 'bar',
  }
  
  const desired: CrossplaneDesiredResources = {
    // Desired composite: use helper to produce a desired-safe XR object
    composite: {
      resource: toDesiredCompositeResource(composite),
      // Example connection detail on the composite (adjust to your needs)
      connectionDetails: {
        example: 'from-js-function',
      },
    },
    // Desired composed resources (existing behaviour)
    resources: {
      configmap: {
        resource: testConfigMap,
        ready: true,
      },
    },
    // Extra resources requested from Crossplane and injected back into the next
    // function run.
    extraResourceRequirements: {
      // 1) Namespaced retrieval: constrained to the XR namespace
      nsConfigMap: {
        apiVersion: 'v1',
        kind: 'ConfigMap',
        namespace: namespace || 'test-xfuncjs',
        matchLabels: {
          'crossplane-js.dev/e2e': 'extra',
          'crossplane-js.dev/scope': 'ns-only',
        },
      },

      // 2) Namespaced resource type, but cluster-wide search across all
      // namespaces (namespace omitted)
      allNsConfigMaps: {
        apiVersion: 'v1',
        kind: 'ConfigMap',
        matchLabels: {
          'crossplane-js.dev/e2e': 'extra',
          'crossplane-js.dev/scope': 'all-ns',
        },
      },

      // 3) Cluster-scoped retrieval
      testNamespace: {
        apiVersion: 'v1',
        kind: 'Namespace',
        matchName: namespace || 'test-xfuncjs',
      },
    },
  }

  // Log what we requested
  logger.info(
    `extraResourceRequirements: ${JSON.stringify(desired.extraResourceRequirements)}`
  )

  // Log what we actually got back from Crossplane
  logger.info(`Injected extraResources: ${JSON.stringify(extraResources)}`)
  
  if (extraResources) {
    const extra = extraResources as Record<string, unknown[]>

    // Log the keys we actually received from Crossplane
    logger.info(`extraResources keys: ${Object.keys(extra).join(', ')}`)

    if (extra.exampleRequirement) {
      logger.info(`exampleRequirement count: ${(extra.exampleRequirement || []).length}`)
    }
  }

  logger.info('SimpleConfigMap composition function completed')
  logger.debug({ desired }, 'Generated output')

  return desired
}
