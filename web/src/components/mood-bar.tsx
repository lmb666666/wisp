import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { useDeleteMood, useSetMood, useTodayMood } from "@/api/moods"
import { MoodPicker } from "@/components/mood-picker"
import type { Mood } from "@/lib/moods"

/** 首页心情条：一次点击即保存，失败回滚。 */
export function MoodBar() {
  const { today, mood } = useTodayMood()
  return <MoodBarInner key={`${today}:${mood ?? "none"}`} today={today} saved={mood} />
}

function MoodBarInner({ today, saved }: { today: string; saved?: Mood }) {
  const { t } = useTranslation()
  const [selected, setSelected] = useState<Mood | undefined>(saved)
  const [pending, setPending] = useState(false)
  const setMood = useSetMood()
  const deleteMood = useDeleteMood()

  const choose = (mood: Mood) => {
    if (pending || mood === selected) return
    const before = selected
    setSelected(mood)
    setPending(true)
    setMood.mutate(
      { date: today, mood },
      {
        onError: () => {
          setSelected(before)
          toast.error(t("mood.saveFailed"))
        },
        onSettled: () => setPending(false),
      },
    )
  }

  const clear = () => {
    if (pending || !selected) return
    const before = selected
    setSelected(undefined)
    setPending(true)
    deleteMood.mutate(today, {
      onError: () => {
        setSelected(before)
        toast.error(t("mood.deleteFailed"))
      },
      onSettled: () => setPending(false),
    })
  }

  return (
    <section className="wisp-card mb-3 px-3 py-2.5" aria-label={t("mood.prompt")}>
      <div className="mb-1.5 flex items-center justify-between gap-2">
        <p className="text-muted-foreground text-xs" aria-live="polite">
          {selected ? t("mood.todayLabel", { mood: t(`mood.${selected}`) }) : t("mood.prompt")}
        </p>
        {selected && (
          <button
            type="button"
            onClick={clear}
            disabled={pending}
            className="text-muted-foreground hover:text-foreground text-[11px] transition-colors disabled:opacity-60"
          >
            {t("mood.clear")}
          </button>
        )}
      </div>
      <MoodPicker value={selected} disabled={pending} onSelect={choose} />
    </section>
  )
}
