import {
  logger,
  // FieldRef,
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
    // Example extra resource requirement (optional – adjust or remove as needed)
    extraResourceRequirements: {
      // This doesn't work and seem to be a problem on the Crossplane side
      // exampleRequirement: {
      //   apiVersion: 'v1',
      //   kind: 'Namespace',
      //   matchLabels: {
      //     "kubernetes.io/metadata.name": "crossplane-system",
      //   }
      // },
      exampleRequirement2: {
        apiVersion: 'v1',
        kind: 'ConfigMap',
        matchLabels: {
          "fou": "barjo",
        }
      }
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
