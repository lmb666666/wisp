import { useNavigate, useSearch } from "@tanstack/react-router"
import { useTranslation } from "react-i18next"

import { useMe } from "@/api/auth"
import { MoodTab } from "@/components/mood-calendar"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { ArchiveList } from "@/routes/stats/archive-list"
import { Heatmap } from "@/routes/stats/heatmap"
import { Overview } from "@/routes/stats/overview"
import { TagCloud } from "@/routes/stats/tag-cloud"

/** 统计中心：概览、动态（热力图 + 归档）、心情、标签。 */
export function StatsPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { data: me } = useMe()
  const { tab } = useSearch({ from: "/stats" })
  const isOwner = !!me?.is_owner
  const current = tab === "overview" && !isOwner ? "stats" : (tab ?? "stats")

  return (
    <div className="wisp-page space-y-4 py-5">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold tracking-tight">{t("nav.stats")}</h1>
        <Tabs
          value={current}
          onValueChange={(v) => navigate({ to: "/stats", search: v === "stats" ? {} : { tab: v } })}
        >
          <TabsList>
            {isOwner && <TabsTrigger value="overview">{t("stats.tabOverview")}</TabsTrigger>}
            <TabsTrigger value="stats">{t("stats.tabStats")}</TabsTrigger>
            <TabsTrigger value="mood">{t("mood.tab")}</TabsTrigger>
            <TabsTrigger value="tags">{t("stats.tabTags")}</TabsTrigger>
          </TabsList>
        </Tabs>
      </div>

      {current === "overview" && <Overview />}
      {current === "stats" && (
        <div className="space-y-5">
          <Heatmap />
          <section className="space-y-3">
            <h2 className="wisp-section-title">{t("stats.tabArchive")}</h2>
            <ArchiveList />
          </section>
        </div>
      )}
      {current === "mood" && <MoodTab isOwner={isOwner} />}
      {current === "tags" && <TagCloud />}
    </div>
  )
}
