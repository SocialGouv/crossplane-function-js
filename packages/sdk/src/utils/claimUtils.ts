import { KubernetesResource } from "../types";

/**
 * Extracts the name from a given composite resource object.
 */
export function getClaimName(claim: KubernetesResource): string {
    const name = claim.metadata.name;
    const claimName = claim.metadata.labels?.["crossplane.io/claim-name"];
    if (claimName) {
        return claimName;
    }
    throw new Error(`Resource ${name} wasn't created via a claim`);
}

/**
 * Extracts the namespace from a given composite resource object.
 */
export function getClaimNamespace(claim: KubernetesResource): string {
    const name = claim.metadata.name;
    const claimNamespace = claim.metadata.labels?.["crossplane.io/claim-namespace"];
    if (claimNamespace) {
        return claimNamespace;
    }
    throw new Error(`Resource ${name} wasn't created via a claim`);
}
