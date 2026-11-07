import { describe, expect, it } from "vitest"

import { splitTagFilter } from "./client"

describe("splitTagFilter", () => {
  it("parses JSON array filters", () => {
    expect(splitTagFilter('["设计","日记"]')).toEqual(["设计", "日记"])
  })

  it("drops non-string entries and empty strings", () => {
    expect(splitTagFilter('[1,"a","",null]')).toEqual(["a"])
  })

  it("falls back to comma-separated legacy values", () => {
    expect(splitTagFilter("a,b")).toEqual(["a", "b"])
  })

  it("returns empty for invalid JSON and missing input", () => {
    expect(splitTagFilter("[oops")).toEqual([])
    expect(splitTagFilter(undefined)).toEqual([])
    expect(splitTagFilter("")).toEqual([])
  })
})
