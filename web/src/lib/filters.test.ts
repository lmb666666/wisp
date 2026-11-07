import { describe, expect, it } from "vitest"

import { activeFilterCount } from "./filters"

describe("activeFilterCount", () => {
  it("counts nothing for empty filters", () => {
    expect(activeFilterCount({})).toBe(0)
  })

  it("counts the date range once and each tag", () => {
    expect(activeFilterCount({ tag: "设计" })).toBe(1)
    expect(activeFilterCount({ tags: '["a","b"]', from: 1 })).toBe(3)
    expect(activeFilterCount({ to: 1 })).toBe(1)
  })

  it("dedupes tag and tags", () => {
    expect(activeFilterCount({ tag: "a", tags: '["a","b"]' })).toBe(2)
  })
})
