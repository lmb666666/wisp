import { afterEach, describe, expect, it } from "vitest"

import { randomUUID } from "./id"

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/

describe("randomUUID", () => {
  afterEach(() => {
    Reflect.deleteProperty(crypto, "randomUUID")
  })

  it("生成 v4 UUID", () => {
    expect(randomUUID()).toMatch(UUID_RE)
  })

  it("安全上下文缺失 randomUUID 时仍能生成", () => {
    Object.defineProperty(crypto, "randomUUID", { configurable: true, value: undefined })
    expect(randomUUID()).toMatch(UUID_RE)
  })
})
