import { GitBranch, Globe2, MapPin, Music4, Video } from "lucide-react"

import type { BlockInput } from "@/components/block-dialog"

const ICONS = {
  music: Music4,
  github: GitBranch,
  website: Globe2,
  video: Video,
  location: MapPin,
} as const

/** 扩展块类型 → 图标（发布框列表与详情页共用）。 */
export function BlockIcon({ type, className }: { type: BlockInput["type"]; className?: string }) {
  const Icon = ICONS[type]
  return <Icon className={className ?? "text-muted-foreground size-4"} />
}
