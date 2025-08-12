import { createLogger } from "@crossplane-js/libs"
import { getRegisteredXrdModelByApiVersion } from "@crossplane-js/sdk"

// Create a logger for this module
const moduleLogger = createLogger("model")

export const createModel = (compositeResource: any): any => {
  let composite = compositeResource
  const RegisteredModelClass = getRegisteredXrdModelByApiVersion(
    compositeResource.apiVersion,
    compositeResource.kind
  )

  if (RegisteredModelClass) {
    moduleLogger.info(`Using registered XRD model for ${compositeResource.kind}`)
    composite = new RegisteredModelClass(compositeResource)
  } else {
    moduleLogger.warn(
      `No registered XRD model found for ${compositeResource.kind}, using raw input`
    )
  }
  return composite
}
