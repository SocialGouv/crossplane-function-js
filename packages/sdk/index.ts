// Re-export logger from @crossplane-js/libs
export { logger, createLogger } from "@crossplane-js/libs"

// Export types
export type * from "./src/types.ts"

export * from "./src/Model/index.ts"
export * from "./src/utils/FieldRef.ts"
export * from "./src/utils/secretUtils.ts"

// Export Kubernetes resources with FieldRef support
export * from "./src/kubernetes/index.ts"
