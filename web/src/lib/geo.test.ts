import { describe, expect, it } from "vitest"

import { formatCoords, mapEmbedURL, osmURL, parseCoords } from "./geo"

describe("parseCoords", () => {
  it("accepts half/full-width commas and whitespace", () => {
    expect(parseCoords("31.2304, 121.4737")).toEqual({ lat: 31.2304, lng: 121.4737 })
    expect(parseCoords(" 31.2304，121.4737 ")).toEqual({ lat: 31.2304, lng: 121.4737 })
    expect(parseCoords("-0.5,-179.9")).toEqual({ lat: -0.5, lng: -179.9 })
  })

  it("rejects malformed or out-of-range values", () => {
    expect(parseCoords("上海")).toBeNull()
    expect(parseCoords("91,0")).toBeNull()
    expect(parseCoords("0,181")).toBeNull()
    expect(parseCoords("1,2,3")).toBeNull()
  })
})

describe("formatCoords", () => {
  it("keeps four decimal places", () => {
    expect(formatCoords({ lat: 31.23456, lng: 121.5 })).toBe("31.2346,121.5000")
  })
})

describe("osmURL", () => {
  it("uses coordinates when available", () => {
    expect(osmURL("家", { lat: 31.2, lng: 121.4 })).toBe(
      "https://www.openstreetmap.org/?mlat=31.2&mlon=121.4#map=15/31.2/121.4",
    )
  })

  it("parses coordinates embedded in the name", () => {
    expect(osmURL("31.2,121.4")).toContain("mlat=31.2&mlon=121.4")
  })

  it("falls back to an encoded search", () => {
    expect(osmURL("上海 徐汇")).toBe(
      "https://www.openstreetmap.org/search?query=" + encodeURIComponent("上海 徐汇"),
    )
  })
})

describe("mapEmbedURL", () => {
  it("encodes bbox and marker", () => {
    const url = new URL(mapEmbedURL({ lat: 31.2, lng: 121.4 }))
    expect(url.origin + url.pathname).toBe("https://www.openstreetmap.org/export/embed.html")
    expect(url.searchParams.get("layer")).toBe("mapnik")
    expect(url.searchParams.get("marker")).toBe("31.2,121.4")

    const bbox = url.searchParams.get("bbox")?.split(",").map(Number) ?? []
    expect(bbox).toHaveLength(4)
    expect(bbox[0]).toBeCloseTo(121.392)
    expect(bbox[1]).toBeCloseTo(31.196)
    expect(bbox[2]).toBeCloseTo(121.408)
    expect(bbox[3]).toBeCloseTo(31.204)
  })
})
