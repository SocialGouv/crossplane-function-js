/**
 * Encode string data to base64 for Kubernetes secret storage
 * @param data - Object with string keys and values to encode
 * @returns Object with base64-encoded values
 */
export const toSecretData = (data: Record<string, string>): Record<string, string> => {
  const result: Record<string, string> = {}
  for (const [key, value] of Object.entries(data)) {
    result[key] = Buffer.from(value, "utf8").toString("base64")
  }
  return result
}

/**
 * Decode base64 data from Kubernetes secret storage
 * @param data - Object with string keys and base64-encoded values
 * @returns Object with decoded string values
 */
export const fromSecretData = (data: Record<string, string>): Record<string, string> => {
  const result: Record<string, string> = {}
  for (const [key, value] of Object.entries(data)) {
    result[key] = Buffer.from(value, "base64").toString("utf8")
  }
  return result
}
