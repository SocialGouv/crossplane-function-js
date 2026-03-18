import { expect, test } from "vitest"

import { getErrorsFromObservedResources } from "./responseUtils.ts"

test("getErrorsFromObservedResources returns errors for kubernetes provider resources with conditions indicating an error", () => {
  const observedResources = {
    resource1: {
      resource: {
        apiVersion: "kubernetes.crossplane.io/v1alpha2",
        kind: "Object",
        metadata: {
          name: "env-01kkz4keyagp3xqn7qdp6zgf9p-argocd-app",
        },
        status: {
          atProvider: {
            manifest: {
              apiVersion: "argoproj.io/v1alpha1",
              kind: "Application",
              metadata: {
                name: "env-01kkz4keyagp3xqn7qdp6zgf9p",
                namespace: "argo-cd",
              },
              status: {
                conditions: [
                  {
                    lastTransitionTime: "2026-03-18T10:56:25Z",
                    message:
                      "application destination server 'dev' and namespace 'env-01kkz4keyagp3xqn7qdp6zgf9p' do not match any of the allowed destinations in project 'env-01kkz4keyagp3xqn7qdp6zgf9p'",
                    type: "InvalidSpecError",
                  },
                ],
                health: {
                  lastTransitionTime: "2026-03-18T10:56:25Z",
                  status: "Unknown",
                },
                sync: {
                  status: "Unknown",
                },
              },
            },
          },
          conditions: [
            {
              lastTransitionTime: "2026-03-18T10:56:26Z",
              reason: "Unavailable",
              status: "False",
              type: "Ready",
            },
            {
              lastTransitionTime: "2026-03-18T13:54:09Z",
              observedGeneration: 2,
              reason: "ReconcileSuccess",
              status: "True",
              type: "Synced",
            },
          ],
        },
      },
    },
  }

  const errors = getErrorsFromObservedResources(observedResources)
  expect(errors.length).toBe(2) // One for the Ready condition, one for the Kubernetes Object conditions
})

test("getErrorsFromObservedResources returns errors for resources with conditions indicating an error", () => {
  const observedResources = {
    resource1: {
      resource: {
        apiVersion: "oss.grafana.crossplane.io/v1alpha1",
        kind: "OrganizationPreferences",
        metadata: {
          name: "test-for-errors",
        },
        status: {
          atProvider: {},
          conditions: [
            {
              lastTransitionTime: "2026-03-18T14:35:48Z",
              observedGeneration: 2,
              reason: "Creating",
              status: "False",
              type: "Ready",
            },
            {
              lastTransitionTime: "2026-03-18T14:35:48Z",
              message:
                'create failed: failed to create the resource: [{0 [PUT /org/preferences][403] updateOrgPreferencesForbidden {"message":"You\'ll need additional permissions to perform this action. Permissions needed: orgs.preferences:write"}  []}]',
              observedGeneration: 2,
              reason: "ReconcileError",
              status: "False",
              type: "Synced",
            },
          ],
        },
      },
    },
    resource2: {
      resource: {
        apiVersion: "identity.vault.upbound.io/v1alpha1",
        kind: "GroupAlias",
        metadata: {
          name: "test-for-errors",
        },
        status: {
          atProvider: {},
          conditions: [
            {
              lastTransitionTime: "2026-03-18T14:39:50Z",
              message:
                "create failed: async create failed: failed to create the resource: [{0 error writing IdentityGroupAlias to \"fakiest\": Error making API request.\n\nURL: PUT https://vault.dev.atlas-sandbox.public-cloud.social.gouv.fr/v1/identity/group-alias\nCode: 400. Errors:\n\n* invalid group ID given in 'canonical_id'  []}]",
              reason: "ReconcileError",
              status: "False",
              type: "Synced",
            },
            {
              lastTransitionTime: "2026-03-18T14:39:50Z",
              observedGeneration: 3,
              reason: "Creating",
              status: "False",
              type: "Ready",
            },
            {
              lastTransitionTime: "2026-03-18T14:39:50Z",
              message:
                "async create failed: failed to create the resource: [{0 error writing IdentityGroupAlias to \"fakiest\": Error making API request.\n\nURL: PUT https://vault.dev.atlas-sandbox.public-cloud.social.gouv.fr/v1/identity/group-alias\nCode: 400. Errors:\n\n* invalid group ID given in 'canonical_id'  []}]",
              reason: "AsyncCreateFailure",
              status: "False",
              type: "LastAsyncOperation",
            },
          ],
        },
      },
    },
  }

  const errors = getErrorsFromObservedResources(observedResources)
  expect(errors.length).toBe(4)
})

test("getErrorsFromObservedResources returns an empty array when there are no conditions indicating an error", () => {
  const observedResources = {
    resource1: {
      resource: {
        apiVersion: "example.com/v1",
        kind: "ExampleResource",
        metadata: {
          name: "example-resource",
        },
        status: {
          conditions: [
            {
              lastTransitionTime: "2026-03-18T10:56:26Z",
              reason: "Available",
              status: "True",
              type: "Ready",
            },
            {
              lastTransitionTime: "2026-03-18T13:54:09Z",
              observedGeneration: 2,
              reason: "ReconcileSuccess",
              status: "True",
              type: "Synced",
            },
          ],
        },
      },
    },
  }

  const errors = getErrorsFromObservedResources(observedResources)
  expect(errors.length).toBe(0)
})

test("getErrorsFromObservedResources returns an empty array when there are no conditions", () => {
  const observedResources = {
    resource1: {
      resource: {
        apiVersion: "example.com/v1",
        kind: "ExampleResource",
        metadata: {
          name: "example-resource",
        },
        // No status or conditions
      },
    },
  }

  const errors = getErrorsFromObservedResources(observedResources)
  expect(errors.length).toBe(0)
})
