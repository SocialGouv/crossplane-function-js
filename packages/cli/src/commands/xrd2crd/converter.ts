import lodash from "lodash"
import YAML from "yaml"

// Template for XR (Composite Resource) schema
const XR_SCHEMA_TEMPLATE = {
  properties: {
    apiVersion: { type: "string" },
    kind: { type: "string" },
    metadata: {
      properties: {
        name: { maxLength: 63, type: "string" },
      },
      type: "object",
    },
    spec: {
      properties: {
        claimRef: {
          properties: {
            apiVersion: { type: "string" },
            kind: { type: "string" },
            name: { type: "string" },
            namespace: { type: "string" },
          },
          required: ["apiVersion", "kind", "namespace", "name"],
          type: "object",
        },
        compositionRef: {
          properties: { name: { type: "string" } },
          required: ["name"],
          type: "object",
        },
        compositionRevisionRef: {
          properties: { name: { type: "string" } },
          required: ["name"],
          type: "object",
        },
        compositionRevisionSelector: {
          properties: {
            matchLabels: {
              additionalProperties: { type: "string" },
              type: "object",
            },
          },
          required: ["matchLabels"],
          type: "object",
        },
        compositionSelector: {
          properties: {
            matchLabels: {
              additionalProperties: { type: "string" },
              type: "object",
            },
          },
          required: ["matchLabels"],
          type: "object",
        },
        compositionUpdatePolicy: {
          default: "Automatic",
          enum: ["Automatic", "Manual"],
          type: "string",
        },
        publishConnectionDetailsTo: {
          properties: {
            configRef: {
              default: { name: "default" },
              properties: { name: { type: "string" } },
              type: "object",
            },
            metadata: {
              properties: {
                annotations: {
                  additionalProperties: { type: "string" },
                  type: "object",
                },
                labels: {
                  additionalProperties: { type: "string" },
                  type: "object",
                },
                type: { type: "string" },
              },
              type: "object",
            },
            name: { type: "string" },
          },
          required: ["name"],
          type: "object",
        },
        resourceRefs: {
          items: {
            properties: {
              apiVersion: { type: "string" },
              kind: { type: "string" },
              name: { type: "string" },
            },
            required: ["apiVersion", "kind"],
            type: "object",
          },
          type: "array",
          "x-kubernetes-list-type": "atomic",
        },
        writeConnectionSecretToRef: {
          properties: {
            name: { type: "string" },
            namespace: { type: "string" },
          },
          required: ["name", "namespace"],
          type: "object",
        },
      },
      type: "object",
    },
    status: {
      properties: {
        claimConditionTypes: {
          items: { type: "string" },
          type: "array",
          "x-kubernetes-list-type": "set",
        },
        conditions: {
          description: "Conditions of the resource.",
          items: {
            properties: {
              lastTransitionTime: {
                format: "date-time",
                type: "string",
              },
              message: { type: "string" },
              reason: { type: "string" },
              status: { type: "string" },
              type: { type: "string" },
            },
            required: ["lastTransitionTime", "reason", "status", "type"],
            type: "object",
          },
          type: "array",
          "x-kubernetes-list-map-keys": ["type"],
          "x-kubernetes-list-type": "map",
        },
        connectionDetails: {
          properties: {
            lastPublishedTime: { format: "date-time", type: "string" },
          },
          type: "object",
        },
      },
      type: "object",
    },
  },
  required: ["spec"],
  type: "object",
}

// Define interfaces for XRD and CRD structures
export interface XRDMetadata {
  name: string
  [key: string]: unknown
}

export interface XRDNames {
  kind: string
  plural: string
  singular: string
  [key: string]: unknown
}

export interface XRDClaimNames {
  kind: string
  plural: string
  singular: string
  [key: string]: unknown
}

export interface XRDVersion {
  name: string
  served: boolean
  referenceable: boolean
  schema: {
    openAPIV3Schema: Record<string, any>
  }
  [key: string]: unknown
}

export interface XRDSpec {
  group: string
  names: XRDNames
  claimNames?: XRDClaimNames
  versions: XRDVersion[]
  [key: string]: unknown
}

export interface XRD {
  apiVersion: string
  kind: string
  metadata: XRDMetadata
  spec: XRDSpec
  [key: string]: unknown
}

export interface CRD {
  apiVersion: string
  kind: string
  metadata: Record<string, any>
  spec: Record<string, any>
  [key: string]: unknown
}

/**
 * Helper function to replace int-or-string format with oneOf schema
 * @param schema The schema to process
 * @returns The processed schema
 */
export function replaceIntOrString(schema: any): any {
  if (typeof schema !== "object" || schema === null) {
    return schema
  }

  if (Array.isArray(schema)) {
    return schema.map(item => replaceIntOrString(item))
  }

  const result: Record<string, any> = {}

  for (const [key, value] of Object.entries(schema)) {
    if (typeof value === "object" && value !== null) {
      if ("format" in value && value.format === "int-or-string") {
        result[key] = {
          oneOf: [{ type: "string" }, { type: "integer" }],
        }
      } else {
        result[key] = replaceIntOrString(value)
      }
    } else {
      result[key] = value
    }
  }

  return result
}

/**
 * Convert an XRD object to a CRD object
 * @param xrd The XRD object to convert
 * @returns The converted CRD object
 */
export function convertXRDtoCRD(xrd: XRD): CRD {
  // Create the basic CRD structure for the XR (Composite Resource)
  const crd: CRD = {
    apiVersion: "apiextensions.k8s.io/v1",
    kind: "CustomResourceDefinition",
    metadata: {
      name: xrd.metadata.name,
      // Add owner reference to the original XRD
      ownerReferences: [
        {
          apiVersion: xrd.apiVersion,
          kind: xrd.kind,
          name: xrd.metadata.name,
          controller: true,
          blockOwnerDeletion: true,
        },
      ],
    },
    spec: {
      group: xrd.spec.group,
      names: {
        ...xrd.spec.names,
        categories: ["composite"],
        listKind: `${xrd.spec.names.kind}List`,
      },
      scope: "Cluster",
      conversion: {
        strategy: "None",
      },
      versions: xrd.spec.versions.map(version => {
        // Deep clone the version schema to avoid modifying the original
        const versionSchema = JSON.parse(JSON.stringify(version.schema.openAPIV3Schema))

        // Replace int-or-string format with oneOf schema
        const processedSchema = replaceIntOrString(versionSchema)

        // Merge the XRD schema with the XR template
        const mergedSchema = lodash.merge({}, processedSchema, XR_SCHEMA_TEMPLATE)

        return {
          name: version.name,
          served: version.served,
          storage: true,
          schema: {
            openAPIV3Schema: mergedSchema,
          },
          additionalPrinterColumns: [
            {
              name: "SYNCED",
              type: "string",
              jsonPath: ".status.conditions[?(@.type=='Synced')].status",
            },
            {
              name: "READY",
              type: "string",
              jsonPath: ".status.conditions[?(@.type=='Ready')].status",
            },
            {
              name: "COMPOSITION",
              type: "string",
              jsonPath: ".spec.compositionRef.name",
            },
            {
              name: "COMPOSITIONREVISION",
              type: "string",
              jsonPath: ".spec.compositionRevisionRef.name",
              priority: 1,
            },
            {
              name: "AGE",
              type: "date",
              jsonPath: ".metadata.creationTimestamp",
            },
          ],
          subresources: {
            status: {},
          },
        }
      }),
    },
  }

  return crd
}

/**
 * Parse and validate an XRD from YAML content
 * @param xrdContent The YAML content of the XRD
 * @returns The parsed and validated XRD object
 */
export function parseAndValidateXRD(xrdContent: string): XRD {
  const xrd = YAML.parse(xrdContent) as XRD

  // Validate the XRD
  if (
    xrd.kind !== "CompositeResourceDefinition" ||
    !xrd.apiVersion.includes("apiextensions.crossplane.io")
  ) {
    throw new Error(
      "Invalid XRD: Expected kind 'CompositeResourceDefinition' and apiVersion 'apiextensions.crossplane.io/*'"
    )
  }

  return xrd
}
