import type { Coords } from "@/lib/geo"

export interface GeocodeHit extends Coords {
  name: string
}

function root(base: string): string {
  return base.trim().replace(/\/+$/, "")
}

function shortName(display: string): string {
  return display
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean)
    .slice(0, 3)
    .join(" · ")
    .slice(0, 120)
}

/** Nominatim 兼容搜索；返回名称 + 坐标。 */
export async function searchPlaces(
  base: string,
  query: string,
  signal?: AbortSignal,
): Promise<GeocodeHit[]> {
  const q = query.trim()
  if (!base || q.length < 2) return []
  const url = `${root(base)}/search?format=jsonv2&limit=5&q=${encodeURIComponent(q)}`
  const res = await fetch(url, { signal, headers: { Accept: "application/json" } })
  if (!res.ok) throw new Error(`geocode ${res.status}`)
  const rows = (await res.json()) as { display_name?: string; lat?: string; lon?: string }[]
  return rows
    .map((r) => ({
      name: shortName(r.display_name ?? ""),
      lat: Number(r.lat),
      lng: Number(r.lon),
    }))
    .filter((r) => r.name && Number.isFinite(r.lat) && Number.isFinite(r.lng))
}

/** 坐标反查地点名；失败返回空串（卡片仍显示坐标）。 */
export async function reversePlace(
  base: string,
  coords: Coords,
  signal?: AbortSignal,
): Promise<string> {
  if (!base) return ""
  const url = `${root(base)}/reverse?format=jsonv2&lat=${coords.lat}&lon=${coords.lng}`
  const res = await fetch(url, { signal, headers: { Accept: "application/json" } })
  if (!res.ok) throw new Error(`reverse geocode ${res.status}`)
  const row = (await res.json()) as { display_name?: string }
  return shortName(row.display_name ?? "")
}
