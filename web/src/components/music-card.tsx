import { Loader2, Music4, Pause, Play } from "lucide-react"
import { useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"

import type { PostBlock } from "@/api/client"
import { BlockShell } from "@/components/block-shell"

const PLAY_EVENT = "wisp:music-play"
const DEFAULT_VOLUME = 0.7

function fmt(v: number) {
  if (!Number.isFinite(v) || v < 0) return "00:00"
  const m = Math.floor(v / 60)
  const s = Math.floor(v % 60)
  return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`
}

/** 音乐卡（参考 Ech0）：封面 + 歌名/歌手 + 进度条 + 播放键，单曲互斥播放。 */
export function MusicCard({ block }: { block: PostBlock }) {
  const { t } = useTranslation()
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const token = useRef({})
  const [playing, setPlaying] = useState(false)
  const [loading, setLoading] = useState(false)
  const [failed, setFailed] = useState(false)
  const [current, setCurrent] = useState(0)
  const [duration, setDuration] = useState(0)
  const [coverOK, setCoverOK] = useState(true)

  const [title, artist] = splitTrack(block.title)
  const audioURL = block.audio_url ?? ""
  const progress = duration > 0 ? Math.min(100, (current / duration) * 100) : 0

  useEffect(() => {
    const onOther = (e: Event) => {
      if ((e as CustomEvent<object>).detail !== token.current) {
        audioRef.current?.pause()
        setPlaying(false)
      }
    }
    window.addEventListener(PLAY_EVENT, onOther)
    return () => window.removeEventListener(PLAY_EVENT, onOther)
  }, [])

  useEffect(() => {
    const audio = audioRef.current
    if (audio) audio.volume = DEFAULT_VOLUME
  }, [])

  const toggle = async () => {
    const audio = audioRef.current
    if (!audio || !audioURL) return
    if (failed) {
      setFailed(false)
      audio.load()
    }
    if (playing || loading) {
      audio.pause()
      setPlaying(false)
      setLoading(false)
      return
    }
    window.dispatchEvent(new CustomEvent(PLAY_EVENT, { detail: token.current }))
    setLoading(true)
    try {
      await audio.play()
    } catch (err) {
      setLoading(false)
      if (!(err instanceof DOMException && ["AbortError", "NotAllowedError"].includes(err.name))) {
        setFailed(true)
      }
    }
  }

  const seek = (v: number) => {
    const audio = audioRef.current
    if (!audio || !Number.isFinite(v)) return
    audio.currentTime = v
    setCurrent(v)
  }

  return (
    <BlockShell
      icon={Music4}
      label={t("blocks.musicHeader")}
      href={block.url}
      action={block.url?.includes("qq.com") ? t("blocks.sourceQQ") : t("blocks.musicSource")}
    >
      <div className="flex items-center gap-3 p-3">
        <div className="bg-muted flex size-13 shrink-0 items-center justify-center overflow-hidden rounded-lg">
          {block.cover && coverOK ? (
            <img
              src={block.cover}
              alt=""
              className="size-full object-cover"
              loading="lazy"
              referrerPolicy="no-referrer"
              onError={() => setCoverOK(false)}
            />
          ) : (
            <Music4 className="text-muted-foreground size-5" />
          )}
        </div>

        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-semibold">{title || t("blocks.musicFallback")}</p>
          <p className="text-muted-foreground mt-0.5 truncate text-xs">
            {artist || t("blocks.musicArtistUnknown")}
          </p>
          <div className="text-muted-foreground mt-1.5 flex items-center gap-2 text-[10px] tabular-nums">
            <span>{fmt(current)}</span>
            <input
              type="range"
              min={0}
              max={duration || 0}
              step={0.1}
              value={current}
              disabled={duration <= 0 || !audioURL}
              aria-label={t("blocks.musicProgress")}
              onChange={(e) => seek(Number(e.target.value))}
              className="wisp-range flex-1"
              style={{ "--wisp-progress": `${progress}%` } as React.CSSProperties}
            />
            <span>{fmt(duration)}</span>
          </div>
        </div>

        <button
          type="button"
          onClick={() => void toggle()}
          disabled={!audioURL}
          aria-label={playing ? t("blocks.pause") : t("blocks.play")}
          title={playing ? t("blocks.pause") : t("blocks.play")}
          className="bg-primary text-primary-foreground shadow-sm grid size-10 shrink-0 place-items-center rounded-full transition-transform hover:bg-primary/90 active:scale-95 disabled:opacity-40"
        >
          {loading ? (
            <Loader2 className="size-4 animate-spin" />
          ) : playing ? (
            <Pause className="size-4" />
          ) : (
            <Play className="size-4 translate-x-px" />
          )}
        </button>
      </div>

      {!audioURL && (
        <p className="text-muted-foreground border-border/60 border-t px-3 py-2 text-xs">
          {t("blocks.musicNoApi")}
        </p>
      )}
      {failed && (
        <p className="text-destructive border-border/60 border-t px-3 py-2 text-xs">
          {t("blocks.musicFailed")}
        </p>
      )}

      {audioURL && (
        <audio
          ref={audioRef}
          src={audioURL}
          preload="metadata"
          className="hidden"
          onPlay={() => {
            setLoading(false)
            setPlaying(true)
          }}
          onPause={() => setPlaying(false)}
          onTimeUpdate={() => setCurrent(audioRef.current?.currentTime ?? 0)}
          onLoadedMetadata={() => setDuration(audioRef.current?.duration ?? 0)}
          onDurationChange={() => setDuration(audioRef.current?.duration ?? 0)}
          onEnded={() => {
            setPlaying(false)
            setCurrent(0)
            if (audioRef.current) audioRef.current.currentTime = 0
          }}
          onError={() => {
            setLoading(false)
            setFailed(true)
          }}
        />
      )}
    </BlockShell>
  )
}

/** "晴天 · 周杰伦" → [晴天, 周杰伦]（后端标题为 name · artist）。 */
function splitTrack(title?: string): [string, string] {
  if (!title) return ["", ""]
  const idx = title.lastIndexOf(" · ")
  if (idx === -1) return [title, ""]
  return [title.slice(0, idx), title.slice(idx + 3)]
}
