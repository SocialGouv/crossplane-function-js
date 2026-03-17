import { expect, test } from "vitest"

import { areFieldRefsBroken, FieldRef } from "./FieldRef.ts"

test("areFieldRefsBroken returns true if any FieldRef cannot resolve", () => {
  const resource = {
    metadata: {
      name: "test-resource",
    },
    spec: {
      field1: new FieldRef<string>({}, "$.spec.field1", "value1"),
    },
  }

  const result = areFieldRefsBroken(resource)
  expect(result).toBe(true)
})

test("areFieldRefsBroken returns false if all FieldRefs can resolve", () => {
  const resource = {
    metadata: {
      name: "test-resource",
    },
    spec: {
      field1: new FieldRef<string>({ field1: "value1" }, "$.field1", "value1"),
    },
  }

  expect(areFieldRefsBroken(resource)).toBe(false)
})

test("areFieldRefsBroken returns false if there are no FieldRefs", () => {
  const resource = {
    metadata: {
      name: "test-resource",
    },
    spec: {
      field1: "value1",
    },
  }

  expect(areFieldRefsBroken(resource)).toBe(false)
})
