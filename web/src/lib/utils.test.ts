import { describe, expect, it } from "vitest"

import { cn } from "./utils"

describe("cn", () => {
  it("merges tailwind classes and resolves conflicts", () => {
    expect(cn("px-2 py-1", "px-4")).toBe("py-1 px-4")
  })

  it("keeps unrelated classes", () => {
    expect(cn("text-sm", "font-medium")).toBe("text-sm font-medium")
  })

  it("drops falsy inputs", () => {
    const hidden = false
    expect(cn("a", hidden && "b", undefined, "c")).toBe("a c")
  })
})
