import type { ReactNode } from "react"

import type { Mood } from "@/lib/moods"

const FACES: Record<Mood, ReactNode> = {
  happy: (
    <>
      <path d="M7.8 10.2c.8-1.1 2.2-1.1 3 0" />
      <path d="M13.2 10.2c.8-1.1 2.2-1.1 3 0" />
      <path d="M8 14.2c1.1 2.1 6.9 2.1 8 0" />
    </>
  ),
  calm: (
    <>
      <path d="M8 10.6h2.6" />
      <path d="M13.4 10.6h2.6" />
      <path d="M9.4 14.8c.8.7 4.4.7 5.2 0" />
    </>
  ),
  okay: (
    <>
      <circle cx="9.3" cy="10.6" r="0.85" fill="currentColor" stroke="none" />
      <circle cx="14.7" cy="10.6" r="0.85" fill="currentColor" stroke="none" />
      <path d="M9 15h6" />
    </>
  ),
  tired: (
    <>
      <path d="M8 10.5c.8.6 1.8.6 2.6 0" />
      <path d="M13.4 10.5c.8.6 1.8.6 2.6 0" />
      <path d="M8.6 15c.85-.7 1.7-.7 2.55 0s1.7.7 2.55 0" />
    </>
  ),
  low: (
    <>
      <circle cx="9.3" cy="10.6" r="0.85" fill="currentColor" stroke="none" />
      <circle cx="14.7" cy="10.6" r="0.85" fill="currentColor" stroke="none" />
      <path d="M8 15.6c1.1-2 6.9-2 8 0" />
    </>
  ),
}

/** 五档心情的自绘表情；不依赖 emoji 字体，跨平台一致。 */
export function MoodIcon({
  mood,
  className,
  title,
}: {
  mood: Mood
  className?: string
  title?: string
}) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      role={title ? "img" : undefined}
      aria-hidden={title ? undefined : true}
    >
      {title ? <title>{title}</title> : null}
      <circle cx="12" cy="12" r="9.2" />
      {FACES[mood]}
    </svg>
  )
}
