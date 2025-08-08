import jsonpath from "jsonpath"

/**
 * A reference to a field in an object, with fallback in case the field is not found.
 * Used to reference a value when declaring a composed resource using JSONPath syntax.
 * If the field is not found, the resource will be set with a wrong value, but it will be paused,
 * until a compose function is called again with the correct values.
 *
 * @example
 * // Access a nested field using JSONPath syntax
 * new FieldRef(data, "$.spec.containers[0].name", "default-name")
 *
 * // Access array elements
 * new FieldRef(data, "$.items[*].metadata.name", [])
 *
 * // Access with conditional expressions
 * new FieldRef(data, "$.spec.template.spec.containers[?(@.name=='main')].image", "default:latest")
 */
export class FieldRef<T> extends String {
  private resolved: boolean

  /**
   * Get a value from an object using JSONPath syntax
   * @param valueContainer The object to query
   * @param path JSONPath expression (e.g., "$.spec.containers[0].name")
   * @param fallback The fallback value if the path is not found
   * @param valueTransformer Optional function to transform the found value
   * @returns A tuple containing [value, resolved] where resolved indicates if the path was found
   */
  static getValue<T>(
    valueContainer: Record<string, any>,
    path: string,
    fallback: T,
    valueTransformer?: (value: any) => T
  ): [T, boolean] {
    const obj = jsonpath.value(valueContainer, path)
    if (obj === undefined) {
      return [fallback, false]
    }
    const transformedValue = valueTransformer ? valueTransformer(obj) : (obj as T)
    return [transformedValue, true]
  }

  /**
   * Create a new FieldRef instance using JSONPath syntax
   * @param valueContainer The object to query
   * @param path JSONPath expression (e.g., "$.spec.containers[0].name")
   * @param fallback The fallback value if the path is not found
   * @param valueTransformer Optional function to transform the found value
   */
  constructor(
    valueContainer: Record<string, any>,
    path: string,
    fallback: T,
    valueTransformer?: (value: any) => T
  ) {
    const value = FieldRef.getValue<T>(valueContainer, path, fallback, valueTransformer)
    super(value[0])
    this.resolved = value[1]
  }

  /**
   * Check if the field reference can be resolved
   * @returns true if the field can be resolved, false otherwise
   */
  canResolve(): boolean {
    return this.resolved
  }
}

/**
 * Type that recursively transforms all string properties to accept either string or FieldRef<string>
 */
type WithFieldRefs<T> = T extends string
  ? string | FieldRef<string>
  : T extends number
    ? number | FieldRef<number>
    : T extends boolean
      ? boolean | FieldRef<boolean>
      : T extends (infer U)[]
        ? WithFieldRefs<U>[]
        : T extends object
          ? { [K in keyof T]: WithFieldRefs<T[K]> }
          : T

/**
 * Factory function that creates a new class allowing FieldRef values in place of primitive types
 * @param BaseClass The original Kubernetes model class
 * @returns A new class that accepts FieldRef instances for primitive properties
 */
export function withFieldRefsClassFactory<T extends new (data?: any) => any>(
  BaseClass: T
): new (data?: WithFieldRefs<ConstructorParameters<T>[0]>) => InstanceType<T> {
  return class extends (BaseClass as any) {
    constructor(data?: WithFieldRefs<ConstructorParameters<T>[0]>) {
      // Process the data to resolve FieldRef instances
      const processedData = processFieldRefs(data)
      super(processedData)
    }

    // Override toJSON to ensure FieldRefs are properly serialized
    toJSON() {
      const json = super.toJSON()
      return processFieldRefs(json)
    }
  } as any
}

/**
 * Recursively process an object to resolve FieldRef instances to their string values
 */
function processFieldRefs(obj: any): any {
  if (obj === null || obj === undefined) {
    return obj
  }

  // If it's a FieldRef, return its string value
  if (obj instanceof FieldRef) {
    return obj.toString()
  }

  // If it's an array, process each element
  if (Array.isArray(obj)) {
    return obj.map(item => processFieldRefs(item))
  }

  // If it's an object, process each property
  if (typeof obj === "object") {
    const processed: any = {}
    for (const key in obj) {
      // if (obj.hasOwnProperty(key)) {
      if (Object.hasOwn(obj, key)) {
        processed[key] = processFieldRefs(obj[key])
      }
    }
    return processed
  }

  // For primitive values, return as-is
  return obj
}
