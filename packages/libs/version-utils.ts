/**
 * Compares two Kubernetes API versions according to Kubernetes versioning hierarchy
 * @param a First version (e.g., "v1", "v1beta1", "v1alpha1")
 * @param b Second version (e.g., "v1", "v1beta1", "v1alpha1")
 * @returns number: positive if a > b, negative if a < b, 0 if equal
 */
export function compareKubernetesVersions(a: string, b: string): number {
  type StabilityType = "stable" | "beta" | "alpha"

  // Parse version components
  const parseVersion = (version: string) => {
    const match = version.match(/^v(\d+)(?:(alpha|beta)(\d+))?$/)
    if (!match) {
      // Fallback for non-standard versions
      return { major: 0, stability: "stable" as StabilityType, minor: 0, original: version }
    }

    const [, major, stability, minor = "0"] = match
    const normalizedStability: StabilityType = (stability as StabilityType) || "stable"

    return {
      major: parseInt(major, 10),
      stability: normalizedStability,
      minor: parseInt(minor, 10),
      original: version,
    }
  }

  const versionA = parseVersion(a)
  const versionB = parseVersion(b)

  // Compare major version first
  if (versionA.major !== versionB.major) {
    return versionA.major - versionB.major
  }

  // Same major version, compare stability
  const stabilityOrder: Record<StabilityType, number> = { stable: 3, beta: 2, alpha: 1 }
  const stabilityDiff = stabilityOrder[versionA.stability] - stabilityOrder[versionB.stability]

  if (stabilityDiff !== 0) {
    return stabilityDiff
  }

  // Same stability, compare minor version
  if (versionA.stability !== "stable") {
    return versionA.minor - versionB.minor
  }

  // Both are stable versions, they're equal
  return 0
}

/**
 * Gets the latest (highest) version from an array of Kubernetes API versions
 * @param versions Array of version objects with 'name' property
 * @returns string The highest version name
 */
export function getLatestKubernetesVersion(versions: Array<{ name: string }>): string {
  if (!versions || versions.length === 0) {
    throw new Error("No versions provided")
  }

  if (versions.length === 1) {
    return versions[0].name
  }

  // Sort versions in descending order (highest first)
  const sortedVersions = versions.map(v => v.name).sort((a, b) => compareKubernetesVersions(b, a)) // b, a for descending order

  return sortedVersions[0]
}
