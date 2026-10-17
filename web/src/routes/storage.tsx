import { useQuery } from "@tanstack/react-query"
import { useState } from "react"
import { toast } from "sonner"
import { useTranslation } from "react-i18next"

import { api, apiError } from "@/api/client"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"

interface StorageSettings {
  backend: "local" | "s3" | "lsky"
  mirror_local: boolean
  s3: {
    endpoint: string
    access_key: string
    secret_key: string
    bucket: string
    region: string
    use_ssl: boolean
    base_url: string
  }
  lsky: { base_url: string; token: string }
}

/** 设置 · 存储：local / S3 兼容 / Lsky Pro 图床，含连通性测试与本地镜像开关。 */
export function StoragePage() {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({
    queryKey: ["storage-settings"],
    queryFn: () => api<StorageSettings>("/admin/settings/storage"),
  })

  if (isLoading || !data) return <p className="text-muted-foreground py-6 text-sm">{t("common.loading")}</p>
  return <StorageForm initial={data} />
}

function StorageForm({ initial }: { initial: StorageSettings }) {
  const { t } = useTranslation()
  const [st, setSt] = useState<StorageSettings>(initial)
  const [busy, setBusy] = useState<"test" | "save" | null>(null)

  const set = (patch: Partial<StorageSettings>) => setSt((s) => ({ ...s, ...patch }))
  const setS3 = (patch: Partial<StorageSettings["s3"]>) => setSt((s) => ({ ...s, s3: { ...s.s3, ...patch } }))
  const setLsky = (patch: Partial<StorageSettings["lsky"]>) => setSt((s) => ({ ...s, lsky: { ...s.lsky, ...patch } }))

  const act = async (kind: "test" | "save") => {
    setBusy(kind)
    try {
      if (kind === "test") {
        const r = await api<{ ok: boolean; message: string }>("/admin/settings/storage/test", {
          method: "POST",
          body: st,
        })
        if (r.ok) toast.success(r.message)
        else toast.error(r.message)
      } else {
        await api("/admin/settings/storage", { method: "PUT", body: st })
        toast.success(t("storage.saved"))
      }
    } catch (err) {
      toast.error(apiError(err).message)
    } finally {
      setBusy(null)
    }
  }

  return (
    <div className="space-y-4">
      <div className="wisp-card space-y-4 p-5">
        <div>
          <h2 className="wisp-section-title">{t("storage.title")}</h2>
          <p className="text-muted-foreground mt-1 text-xs leading-relaxed">{t("storage.hint")}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          {(
            [
              ["local", t("storage.local")],
              ["s3", t("storage.s3")],
              ["lsky", t("storage.lsky")],
            ] as const
          ).map(([v, label]) => (
            <button
              key={v}
              type="button"
              onClick={() => set({ backend: v })}
              className={
                "border-border rounded-md border px-3 py-1.5 text-sm " +
                (st.backend === v ? "border-primary bg-primary/10 font-medium" : "hover:bg-accent")
              }
            >
              {label}
            </button>
          ))}
        </div>

        {st.backend === "s3" && (
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label={t("storage.endpoint")} value={st.s3.endpoint} onChange={(v) => setS3({ endpoint: v })} />
            <Field label={t("storage.bucket")} value={st.s3.bucket} onChange={(v) => setS3({ bucket: v })} />
            <Field label={t("storage.accessKey")} value={st.s3.access_key} onChange={(v) => setS3({ access_key: v })} />
            <Field label={t("storage.secretKey")} value={st.s3.secret_key} onChange={(v) => setS3({ secret_key: v })} type="password" />
            <Field label={t("storage.region")} value={st.s3.region} onChange={(v) => setS3({ region: v })} />
            <Field label={t("storage.baseUrl")} value={st.s3.base_url} onChange={(v) => setS3({ base_url: v })} />
            <label className="text-muted-foreground flex items-center gap-2 text-sm">
              <Switch checked={st.s3.use_ssl} onCheckedChange={(v) => setS3({ use_ssl: v })} />
              {t("storage.useSSL")}
            </label>
          </div>
        )}

        {st.backend === "lsky" && (
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label={t("storage.lskyBase")} value={st.lsky.base_url} onChange={(v) => setLsky({ base_url: v })} />
            <Field label={t("storage.lskyToken")} value={st.lsky.token} onChange={(v) => setLsky({ token: v })} type="password" />
          </div>
        )}

        <label className="text-muted-foreground flex items-center gap-2 text-sm">
          <Switch checked={st.mirror_local} onCheckedChange={(v) => set({ mirror_local: v })} />
          {t("storage.mirror")}
        </label>

        <div className="flex items-center gap-2">
          <Button size="sm" variant="outline" onClick={() => act("test")} disabled={busy !== null}>
            {busy === "test" ? t("storage.testing") : t("storage.test")}
          </Button>
          <Button size="sm" onClick={() => act("save")} disabled={busy !== null}>
            {busy === "save" ? t("storage.saving") : t("storage.save")}
          </Button>
        </div>
      </div>
    </div>
  )
}

function Field({
  label,
  value,
  onChange,
  type = "text",
}: {
  label: string
  value: string
  onChange: (v: string) => void
  type?: string
}) {
  return (
    <div className="space-y-1.5">
      <Label>{label}</Label>
      <Input value={value} type={type} onChange={(e) => onChange(e.target.value)} />
    </div>
  )
}
