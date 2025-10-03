import {
  logger,
  // FieldRef,
  v1,
} from '@crossplane-js/sdk'
import type {
  CrossplaneDesiredResources,
  // CrossplaneObservedResources
} from '@crossplane-js/sdk'

import type { SimpleConfigMap } from '../../models/test.crossplane.io/v1beta1'

export default function (
  composite: SimpleConfigMap
  // resources: CrossplaneObservedResources
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
    transformedData[key.toUpperCase()] = value.toUpperCase()
  })

  const testConfigMap = new v1.ConfigMap({
    metadata: {
      name: 'generated-configmap',
      namespace: namespace || 'test-xfuncjs',
    },
    data: transformedData,
  })

  const desired: CrossplaneDesiredResources = {
    resources: {
      configmap: {
        resource: testConfigMap,
        ready: true,
      },
    },
  }

  logger.info('SimpleConfigMap composition function completed')
  logger.debug({ desired }, 'Generated output')

  return desired
}
