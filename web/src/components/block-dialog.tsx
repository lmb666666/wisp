import { useQuery } from "@tanstack/react-query"
import { GitBranch, Globe2, Loader2, MapPin, Music4, Search, Video } from "lucide-react"
import { useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"

import { api } from "@/api/client"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { reversePlace, searchPlaces, type GeocodeHit } from "@/lib/geocode"
import { formatCoords, parseCoords, type Coords } from "@/lib/geo"
import { videoEmbed } from "@/lib/video"

export type BlockKind = "music" | "github" | "location" | "website" | "video"

export interface BlockInput {
  type: BlockKind
  url?: string
  name?: string
  lat?: number
  lng?: number
}

const KIND_META = {
  music: { icon: Music4 },
  github: { icon: GitBranch },
  location: { icon: MapPin },
  website: { icon: Globe2 },
  video: { icon: Video },
} as const

/** 扩展卡片输入弹窗（shadcn Dialog，替代原生 prompt）。 */
export function BlockDialog({
  kind,
  open,
  onOpenChange,
  onSubmit,
}: {
  kind: BlockKind | null
  open: boolean
  onOpenChange: (v: boolean) => void
  onSubmit: (block: BlockInput) => void
}) {
  if (!kind) return null
  return (
    <BlockDialogInner
      key={`${kind}-${open}`}
      kind={kind}
      open={open}
      onOpenChange={onOpenChange}
      onSubmit={onSubmit}
    />
  )
}

function BlockDialogInner({
  kind,
  open,
  onOpenChange,
  onSubmit,
}: {
  kind: BlockKind
  open: boolean
  onOpenChange: (v: boolean) => void
  onSubmit: (block: BlockInput) => void
}) {
  const { t } = useTranslation()
  const [value, setValue] = useState("")
  const [coords, setCoords] = useState<Coords | null>(null)
  const [locating, setLocating] = useState(false)
  const [searching, setSearching] = useState(false)
  const [hits, setHits] = useState<GeocodeHit[]>([])
  const [error, setError] = useState("")
  const searchCtrl = useRef<AbortController | null>(null)
  const reverseCtrl = useRef<AbortController | null>(null)
  useEffect(
    () => () => {
      searchCtrl.current?.abort()
      reverseCtrl.current?.abort()
    },
    [],
  )

  const geo = useQuery({
    queryKey: ["geo-settings"],
    queryFn: () => api<{ geocode_url: string }>("/admin/settings/geo"),
    enabled: kind === "location",
    staleTime: Infinity,
  })
  const geocodeBase = geo.data?.geocode_url ?? ""

  const meta = KIND_META[kind]
  const Icon = meta.icon

  const locate = () => {
    if (!navigator.geolocation) {
      setError(t("blocks.geoUnsupported"))
      return
    }
    setLocating(true)
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        const c = { lat: pos.coords.latitude, lng: pos.coords.longitude }
        const coordText = formatCoords(c)
        setCoords(c)
        setValue(coordText)
        setLocating(false)
        if (geocodeBase) {
          reverseCtrl.current?.abort()
          const ctrl = new AbortController()
          reverseCtrl.current = ctrl
          reversePlace(geocodeBase, c, ctrl.signal)
            .then((name) => {
              if (name) setValue((prev) => (prev === coordText ? name : prev))
            })
            .catch(() => {
              // 反查失败/被取消时保留坐标
            })
        }
      },
      () => {
        setError(t("blocks.geoFailed"))
        setLocating(false)
      },
      { timeout: 8000 },
    )
  }

  const search = async () => {
    if (!geocodeBase) return
    searchCtrl.current?.abort()
    const ctrl = new AbortController()
    searchCtrl.current = ctrl
    setSearching(true)
    setError("")
    try {
      const results = await searchPlaces(geocodeBase, value, ctrl.signal)
      if (searchCtrl.current === ctrl) setHits(results)
    } catch (err) {
      if (!(err instanceof DOMException && err.name === "AbortError")) {
        setError(t("blocks.geoSearchFailed"))
      }
    } finally {
      if (searchCtrl.current === ctrl) setSearching(false)
    }
  }

  const submit = (e: React.FormEvent) => {
    e.preventDefault()
    const v = value.trim()
    if (!v) return
    const hasScheme = /^https?:\/\//i.test(v)
    const withScheme = hasScheme ? v : "https://" + v
    if (kind === "music") {
      if (!/^https?:\/\//.test(v)) {
        setError(t("blocks.invalidURL"))
        return
      }
      let parsed: URL
      try {
        parsed = new URL(v)
      } catch {
        setError(t("blocks.invalidURL"))
        return
      }
      const host = parsed.hostname.toLowerCase()
      const neteaseSong =
        host === "music.163.com" &&
        (parsed.pathname.includes("/song") ||
          parsed.hash.includes("/song") ||
          parsed.pathname.includes("/m/song"))
      const qqSong =
        host === "y.qq.com" &&
        (parsed.pathname.includes("songDetail") ||
          parsed.searchParams.has("songmid") ||
          parsed.searchParams.has("songid"))
      if (!neteaseSong && !qqSong) {
        setError(t("blocks.songOnly"))
        return
      }
      onSubmit({ type: "music", url: v })
    } else if (kind === "github") {
      const clean = v.replace(/^https?:\/\/github\.com\//, "").replace(/\/+$/, "")
      if (!/^[\w.-]+\/[\w.-]+$/.test(clean)) {
        setError(t("blocks.invalidRepo"))
        return
      }
      onSubmit({ type: "github", url: "https://github.com/" + clean })
    } else if (kind === "website") {
      try {
        const u = new URL(withScheme)
        if ((u.protocol !== "http:" && u.protocol !== "https:") || !u.hostname)
          throw new Error("bad")
      } catch {
        setError(t("blocks.invalidURL"))
        return
      }
      onSubmit({ type: "website", url: withScheme })
    } else if (kind === "video") {
      if (!videoEmbed(withScheme)) {
        setError(t("blocks.videoOnly"))
        return
      }
      onSubmit({ type: "video", url: withScheme })
    } else {
      const c = coords ?? parseCoords(v)
      onSubmit({
        type: "location",
        name: c && !coords ? formatCoords(c) : v,
        lat: c?.lat,
        lng: c?.lng,
      })
    }
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Icon className="size-4" /> {t(`blocks.${kind}Title`)}
          </DialogTitle>
          <DialogDescription>{t(`blocks.${kind}Desc`)}</DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="space-y-1.5">
            <Label htmlFor="block-input">
              {kind === "location" ? t("blocks.locationName") : t("blocks.url")}
            </Label>
            <div className="flex gap-2">
              <Input
                id="block-input"
                value={value}
                onChange={(e) => {
                  searchCtrl.current?.abort()
                  reverseCtrl.current?.abort()
                  setValue(e.target.value)
                  setCoords(null)
                  setHits([])
                }}
                placeholder={
                  kind === "music"
                    ? t("blocks.musicPlaceholder")
                    : kind === "github"
                      ? t("blocks.githubPlaceholder")
                      : kind === "website"
                        ? t("blocks.websitePlaceholder")
                        : kind === "video"
                          ? t("blocks.videoPlaceholder")
                          : t("blocks.locationPlaceholder")
                }
                className="bg-muted/40 h-10 rounded-xl"
                inputMode={kind === "location" ? "text" : "url"}
                autoCapitalize="none"
                autoCorrect="off"
                spellCheck={false}
                autoFocus
              />
              {kind === "location" && geocodeBase && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="shrink-0 rounded-lg"
                  disabled={searching || value.trim().length < 2}
                  onClick={() => void search()}
                >
                  {searching ? <Loader2 className="animate-spin" /> : <Search />}
                  {t("blocks.geoSearch")}
                </Button>
              )}
            </div>
          </div>
          {kind === "location" && hits.length > 0 && (
            <div className="border-border/70 max-h-40 overflow-y-auto rounded-lg border p-1">
              {hits.map((hit) => (
                <button
                  key={`${hit.name}-${hit.lat}-${hit.lng}`}
                  type="button"
                  className="hover:bg-muted w-full rounded-md px-2 py-1.5 text-left text-xs"
                  onClick={() => {
                    setValue(hit.name)
                    setCoords({ lat: hit.lat, lng: hit.lng })
                    setHits([])
                  }}
                >
                  {hit.name}
                </button>
              ))}
            </div>
          )}
          {kind === "location" && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="rounded-lg"
              onClick={locate}
              disabled={locating}
            >
              {locating ? <Loader2 className="animate-spin" /> : <MapPin />}
              {t("blocks.useCurrent")}
            </Button>
          )}
          {error && <p className="text-destructive text-sm">{error}</p>}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
              {t("common.cancel")}
            </Button>
            <Button type="submit" disabled={!value.trim()}>
              {t("blocks.add")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
