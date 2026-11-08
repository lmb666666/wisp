import { useTranslation } from "react-i18next"

import { MoodIcon } from "@/components/mood-icon"
import { MOODS, type Mood } from "@/lib/moods"
import { cn } from "@/lib/utils"

interface MoodPickerProps {
  value?: Mood
  disabled?: boolean
  className?: string
  /** 无障碍分组名；默认「今天心情如何？」 */
  label?: string
  onSelect: (mood: Mood) => void
}

/** 五档心情选择器：图标在上、两字标签在下，320px 也能一行放下。 */
export function MoodPicker({ value, disabled, className, label, onSelect }: MoodPickerProps) {
  const { t } = useTranslation()

  return (
    <div
      className={cn("grid grid-cols-5 gap-1", className)}
      role="group"
      aria-label={label ?? t("mood.prompt")}
    >
      {MOODS.map((mood) => {
        const text = t(`mood.${mood}`)
        const active = value === mood
        return (
          <button
            key={mood}
            type="button"
            data-mood={mood}
            data-active={active}
            aria-pressed={active}
            aria-label={text}
            title={text}
            disabled={disabled}
            onClick={() => onSelect(mood)}
            className="mood-option text-muted-foreground flex min-h-11 min-w-0 flex-col items-center justify-center gap-1 rounded-xl px-1 py-1 text-[10px] leading-none transition-colors focus-visible:ring-2 focus-visible:ring-[var(--ring)] focus-visible:outline-none disabled:opacity-60 motion-reduce:transition-none"
          >
            <MoodIcon mood={mood} className="size-[18px]" />
            <span>{text}</span>
          </button>
        )
      })}
    </div>
  )
}
