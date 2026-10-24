import { useQuery, useQueryClient } from "@tanstack/react-query"
import { Music4, Webhook } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { api, apiError } from "@/api/client"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Checkbox } from "@/components/ui/checkbox"

interface WebhookRow {
  id: string
  url: string
  events: string
  created_at: number
}

const EVENTS = ["post.created", "post.updated", "post.deleted"] as const

/** 设置 · 集成：Meting 音乐接口、Webhook。 */
export function WebhooksPage() {
  return (
    <div className="space-y-4">
      <MetingCard />
      <WebhookCard />
    </div>
  )
}

function MetingCard() {
  const { t } = useTranslation()
  const { data, isError } = useQuery({
    queryKey: ["meting"],
    queryFn: () => api<{ url: string }>("/admin/settings/meting"),
  })
  if (isError) {
    return (
      <section className="wisp-card p-5">
        <p className="text-muted-foreground text-sm">{t("common.loadFailed")}</p>
      </section>
    )
  }
  if (!data) {
    return (
      <section className="wisp-card p-5">
        <p className="text-muted-foreground text-sm">{t("common.loading")}</p>
      </section>
    )
  }
  return <MetingForm initial={data.url ?? ""} />
}

function MetingForm({ initial }: { initial: string }) {
  const { t } = useTranslation()
  const [url, setUrl] = useState(initial)
  const [busy, setBusy] = useState(false)

  const save = async () => {
    setBusy(true)
    try {
      await api("/admin/settings/meting", { method: "PUT", body: { url: url.trim() } })
      toast.success(t("settings.saved"))
    } catch (err) {
      toast.error(apiError(err).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="wisp-card p-5">
      <h2 className="flex items-center gap-1.5 wisp-section-title">
        <Music4 className="size-4" /> {t("settings.metingTitle")}
      </h2>
      <p className="text-muted-foreground mt-1 mb-2 text-xs leading-relaxed">{t("settings.metingHint")}</p>
      <pre className="bg-muted/60 text-muted-foreground mb-3 rounded-lg p-2.5 text-[11px] leading-relaxed whitespace-pre-wrap break-all">
        {t("settings.metingExample")}
      </pre>
      <div className="flex flex-wrap items-end gap-2">
        <div className="min-w-56 flex-1 space-y-1.5">
          <Label htmlFor="meting-url">{t("settings.metingUrl")}</Label>
          <Input
            id="meting-url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="https://music.example.com/?server=:server&type=:type&id=:id&r=:r"
            className="rounded-lg"
          />
        </div>
        <Button size="sm" variant="outline" className="rounded-lg" onClick={save} disabled={busy}>
          {t("post.save")}
        </Button>
      </div>
    </section>
  )
}

function WebhookCard() {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const { data, isLoading } = useQuery({
    queryKey: ["webhooks"],
    queryFn: () => api<{ items: WebhookRow[] }>("/admin/webhooks"),
  })
  const [url, setUrl] = useState("")
  const [selected, setSelected] = useState<string[]>(["post.created"])
  const [secret, setSecret] = useState("")
  const [busy, setBusy] = useState(false)

  const toggleEvent = (e: string) =>
    setSelected((s) => (s.includes(e) ? s.filter((x) => x !== e) : [...s, e]))

  const create = async (e2: React.FormEvent) => {
    e2.preventDefault()
    setBusy(true)
    try {
      const r = await api<{ secret: string }>("/admin/webhooks", {
        method: "POST",
        body: { url: url.trim(), events: selected.join(",") },
      })
      setSecret(r.secret)
      setUrl("")
      toast.success(t("settings.webhookCreated"))
      qc.invalidateQueries({ queryKey: ["webhooks"] })
    } catch (err) {
      toast.error(apiError(err).message)
    } finally {
      setBusy(false)
    }
  }

  const remove = async (id: string) => {
    try {
      await api(`/admin/webhooks/${id}`, { method: "DELETE" })
      toast.success(t("settings.deleted"))
      qc.invalidateQueries({ queryKey: ["webhooks"] })
    } catch (err) {
      toast.error(apiError(err).message)
    }
  }

  return (
    <section className="wisp-card p-5">
      <h2 className="flex items-center gap-1.5 wisp-section-title">
        <Webhook className="size-4" /> {t("webhooks.title")}
      </h2>
      <p className="text-muted-foreground mt-1 mb-3 text-xs leading-relaxed">{t("webhooks.hint")}</p>

      <form onSubmit={create} className="space-y-3">
        <div className="space-y-1.5">
          <Label htmlFor="hook-url">{t("webhooks.url")}</Label>
          <Input
            id="hook-url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder={t("webhooks.urlPlaceholder")}
            className="rounded-lg"
          />
        </div>
        <div className="flex flex-wrap items-center gap-4">
          <Label>{t("webhooks.events")}</Label>
          {EVENTS.map((e) => (
            <label key={e} className="text-muted-foreground flex cursor-pointer items-center gap-2 text-sm">
              <Checkbox checked={selected.includes(e)} onCheckedChange={() => toggleEvent(e)} />
              <code className="text-xs">{e}</code>
            </label>
          ))}
        </div>
        <Button type="submit" size="sm" className="rounded-lg" disabled={busy || !url.trim() || selected.length === 0}>
          {busy ? t("webhooks.creating") : t("webhooks.create")}
        </Button>
        {secret && (
          <div className="bg-muted/60 rounded-lg p-3">
            <p className="mb-1 text-xs font-medium">{t("webhooks.secretOnce")}</p>
            <code className="break-all text-xs select-all">{secret}</code>
          </div>
        )}
      </form>

      <div className="mt-4 border-t pt-3">
        <h3 className="mb-1 text-sm font-medium">{t("settings.configuredWebhooks")}</h3>
        {isLoading && <p className="text-muted-foreground text-sm">{t("common.loading")}</p>}
        <ul className="divide-border divide-y">
          {data?.items.map((w) => (
            <li key={w.id} className="flex items-center justify-between gap-3 py-2.5">
              <div className="min-w-0">
                <p className="truncate text-sm">{w.url}</p>
                <p className="text-muted-foreground text-xs">
                  {new Date(w.created_at * 1000).toLocaleDateString()}
                </p>
              </div>
              <div className="flex items-center gap-2">
                {w.events.split(",").map((e) => (
                  <Badge key={e} variant="outline">
                    {e}
                  </Badge>
                ))}
                <Button variant="ghost" size="sm" onClick={() => remove(w.id)}>
                  {t("common.delete")}
                </Button>
              </div>
            </li>
          ))}
          {(data?.items.length ?? 0) === 0 && !isLoading && (
            <p className="text-muted-foreground py-2 text-sm">{t("settings.noWebhooks")}</p>
          )}
        </ul>
      </div>
    </section>
  )
}

