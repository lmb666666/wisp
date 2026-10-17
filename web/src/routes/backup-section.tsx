import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Clock3, HardDriveDownload, Loader2, Play } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { api, apiError } from "@/api/client"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"

interface BackupSettings {
  enabled: boolean
  interval_hours: number
  retention: number
  last_run_at: number
}

interface BackupFile {
  name: string
  size: number
  created_at: number
}

interface BackupResp {
  settings: BackupSettings
  backups: BackupFile[]
}

function fmtBytes(n: number): string {
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)}KB`
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)}MB`
  return `${(n / 1024 / 1024 / 1024).toFixed(2)}GB`
}

/** 设置 · 数据：自动备份（数据库快照）设置与手动运行。 */
export function BackupSection() {
  const { t } = useTranslation()
  const q = useQuery({
    queryKey: ["backup-settings"],
    queryFn: () => api<BackupResp>("/admin/settings/backup"),
  })
  if (q.isLoading)
    return <p className="text-muted-foreground py-6 text-sm">{t("common.loading")}</p>
  if (!q.data) return null
  return <BackupForm initial={q.data.settings} backups={q.data.backups} />
}

function BackupForm({ initial, backups }: { initial: BackupSettings; backups: BackupFile[] }) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [enabled, setEnabled] = useState(initial.enabled)
  const [interval, setIntervalHours] = useState(String(initial.interval_hours))
  const [retention, setRetention] = useState(String(initial.retention))

  const invalidate = () => void qc.invalidateQueries({ queryKey: ["backup-settings"] })
  const save = useMutation({
    mutationFn: () =>
      api<BackupSettings>("/admin/settings/backup", {
        method: "PUT",
        body: {
          enabled,
          interval_hours: Number(interval) || 24,
          retention: Number(retention) || 7,
        },
      }),
    onSuccess: (saved) => {
      setEnabled(saved.enabled)
      setIntervalHours(String(saved.interval_hours))
      setRetention(String(saved.retention))
      toast.success(t("data.backupSaved"))
      invalidate()
    },
    onError: (err) => toast.error(apiError(err).message),
  })
  const runNow = useMutation({
    mutationFn: () => api<BackupFile>("/admin/backup/run", { method: "POST" }),
    onSuccess: (file) => {
      toast.success(t("data.backupRan", { name: file.name }))
      invalidate()
    },
    onError: (err) => toast.error(apiError(err).message),
  })

  const latest = backups[0]
  const dirty =
    enabled !== initial.enabled ||
    Number(interval) !== initial.interval_hours ||
    Number(retention) !== initial.retention

  return (
    <section className="wisp-card p-5">
      <h2 className="wisp-section-title flex items-center gap-1.5">
        <HardDriveDownload className="size-4" /> {t("data.backupTitle")}
      </h2>
      <p className="wisp-hint mt-1">{t("data.backupScheduleHint")}</p>

      <div className="mt-3 flex items-center gap-2 text-sm">
        <Switch
          checked={enabled}
          onCheckedChange={setEnabled}
          aria-label={t("data.backupEnabled")}
        />
        {t("data.backupEnabled")}
      </div>

      <div className="mt-3 grid grid-cols-2 gap-3">
        <label className="text-muted-foreground text-xs">
          {t("data.backupInterval")}
          <div className="mt-1 flex items-center gap-1.5">
            <Input
              type="number"
              min={1}
              max={720}
              value={interval}
              onChange={(e) => setIntervalHours(e.target.value)}
              disabled={!enabled || save.isPending}
              className="h-8"
            />
            <span className="shrink-0">{t("data.backupHours")}</span>
          </div>
        </label>
        <label className="text-muted-foreground text-xs">
          {t("data.backupRetention")}
          <div className="mt-1 flex items-center gap-1.5">
            <Input
              type="number"
              min={1}
              max={365}
              value={retention}
              onChange={(e) => setRetention(e.target.value)}
              disabled={!enabled || save.isPending}
              className="h-8"
            />
            <span className="shrink-0">{t("data.backupCopies")}</span>
          </div>
        </label>
      </div>

      <p className="text-muted-foreground mt-3 flex items-center gap-1.5 text-xs">
        <Clock3 className="size-3.5" />
        {initial.last_run_at > 0
          ? t("data.backupLast", { time: new Date(initial.last_run_at * 1000).toLocaleString() })
          : t("data.backupNever")}
        {latest &&
          ` · ${t("data.backupLatest", { name: latest.name, size: fmtBytes(latest.size) })}`}
      </p>

      <div className="mt-3 flex flex-wrap items-center gap-2">
        <Button size="sm" disabled={!dirty || save.isPending} onClick={() => save.mutate()}>
          {save.isPending && <Loader2 className="animate-spin" />} {t("data.backupSave")}
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={runNow.isPending}
          onClick={() => runNow.mutate()}
        >
          {runNow.isPending ? <Loader2 className="animate-spin" /> : <Play />} {t("data.backupRun")}
        </Button>
        <Button asChild size="sm" variant="ghost">
          <a href="/api/v1/admin/backup" download>
            {t("data.backupButton")}
          </a>
        </Button>
      </div>
      <p className="wisp-hint mt-2">{t("data.backupMediaHint")}</p>
    </section>
  )
}
