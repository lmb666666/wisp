export interface Coords {
  lat: number
  lng: number
}

export function parseCoords(raw: string): Coords | null {
  const m = /^(-?\d+(?:\.\d+)?)\s*[,，]\s*(-?\d+(?:\.\d+)?)$/.exec(raw.trim())
  if (!m) return null
  const lat = Number(m[1])
  const lng = Number(m[2])
  if (
    !Number.isFinite(lat) ||
    !Number.isFinite(lng) ||
    lat < -90 ||
    lat > 90 ||
    lng < -180 ||
    lng > 180
  ) {
    return null
  }
  return { lat, lng }
}

export function formatCoords(c: Coords): string {
  return `${c.lat.toFixed(4)},${c.lng.toFixed(4)}`
}

/** OSM 坐标页（无坐标时退回搜索）。 */
export function osmURL(name: string, coords?: Coords | null): string {
  if (coords) {
    return `https://www.openstreetmap.org/?mlat=${coords.lat}&mlon=${coords.lng}#map=15/${coords.lat}/${coords.lng}`
  }
  const parsed = parseCoords(name)
  if (parsed) return osmURL(name, parsed)
  return "https://www.openstreetmap.org/search?query=" + encodeURIComponent(name)
}

/** 点击后才加载的 OSM embed（bbox 约 ±0.008°，移动端仍可操作）。 */
export function mapEmbedURL(coords: Coords): string {
  const d = 0.008
  const bbox = `${coords.lng - d},${coords.lat - d / 2},${coords.lng + d},${coords.lat + d / 2}`
  return (
    "https://www.openstreetmap.org/export/embed.html?bbox=" +
    encodeURIComponent(bbox) +
    "&layer=mapnik&marker=" +
    encodeURIComponent(`${coords.lat},${coords.lng}`)
  )
}
