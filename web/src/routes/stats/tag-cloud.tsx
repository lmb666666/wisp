import { Link } from "@tanstack/react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Pencil, Trash2 } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { useMe } from "@/api/auth"
import { api, apiError } from "@/api/client"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"

interface TagRow {
  id: number
  name: string
  post_count: number
}

/** 标签云：浏览 + owner 的重命名/合并/删除。 */
export function TagCloud() {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const { data: me } = useMe()
  const [editing, setEditing] = useState(false)
  const [selected, setSelected] = useState<number[]>([])
  const [mergeInto, setMergeInto] = useState("")
  const [mergeOpen, setMergeOpen] = useState(false)
  const [renameTarget, setRenameTarget] = useState<TagRow | null>(null)
  const [renameValue, setRenameValue] = useState("")
  const [deleteTarget, setDeleteTarget] = useState<TagRow | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ["tags"],
    queryFn: () => api<{ items: TagRow[] }>("/tags"),
  })
  const tags = data?.items ?? []

  const invalidate = () => {
    void qc.invalidateQueries({ queryKey: ["tags"] })
    void qc.invalidateQueries({ queryKey: ["posts"] })
    void qc.invalidateQueries({ queryKey: ["post"] })
  }

  const rename = useMutation({
    mutationFn: (input: { id: number; name: string }) =>
      api(`/tags/${input.id}`, { method: "PATCH", body: { name: input.name } }),
    onSuccess: () => {
      toast.success(t("stats.renamed"))
      setRenameTarget(null)
      invalidate()
    },
    onError: (err) => toast.error(apiError(err).message),
  })
  const merge = useMutation({
    mutationFn: (input: { from_ids: number[]; into: string }) =>
      api("/tags/merge", { method: "POST", body: input }),
    onSuccess: () => {
      toast.success(t("stats.merged"))
      setMergeOpen(false)
      setSelected([])
      setMergeInto("")
      invalidate()
    },
    onError: (err) => toast.error(apiError(err).message),
  })
  const remove = useMutation({
    mutationFn: (id: number) => api(`/tags/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      toast.success(t("stats.tagDeleted"))
      setDeleteTarget(null)
      setSelected((prev) => prev.filter((id) => id !== deleteTarget?.id))
      invalidate()
    },
    onError: (err) => toast.error(apiError(err).message),
  })

  const toggle = (id: number) =>
    setSelected((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]))

  if (isLoading) return <p className="text-muted-foreground text-sm">{t("common.loading")}</p>

  return (
    <section className="wisp-card p-4">
      {me?.is_owner && (
        <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
          <p className="wisp-hint">{editing ? t("stats.tagsEditHint") : t("stats.tagsHint")}</p>
          <Button
            variant={editing ? "secondary" : "outline"}
            size="sm"
            onClick={() => {
              setEditing((v) => !v)
              setSelected([])
            }}
          >
            {editing ? (
              t("common.done")
            ) : (
              <>
                <Pencil /> {t("stats.manageTags")}
              </>
            )}
          </Button>
        </div>
      )}

      {editing && selected.length > 0 && (
        <div className="bg-muted/50 mb-3 flex flex-wrap items-center gap-2 rounded-xl p-2">
          <span className="text-muted-foreground text-xs">
            {t("stats.selectedCount", { count: selected.length })}
          </span>
          <Input
            value={mergeInto}
            onChange={(e) => setMergeInto(e.target.value)}
            placeholder={t("stats.mergePlaceholder")}
            aria-label={t("stats.mergeInto")}
            className="h-8 max-w-[220px] flex-1"
          />
          <Button size="sm" disabled={!mergeInto.trim()} onClick={() => setMergeOpen(true)}>
            {t("stats.mergeAction")}
          </Button>
        </div>
      )}

      {editing ? (
        <ul className="divide-border/60 divide-y">
          {tags.map((tg) => (
            <li key={tg.id} className="flex items-center gap-2 py-1.5">
              <Checkbox
                checked={selected.includes(tg.id)}
                onCheckedChange={() => toggle(tg.id)}
                aria-label={tg.name}
              />
              <span className="min-w-0 flex-1 truncate text-sm">#{tg.name}</span>
              <span className="text-muted-foreground text-xs tabular-nums">{tg.post_count}</span>
              <Button
                variant="ghost"
                size="icon"
                className="size-7"
                aria-label={t("stats.renameTag", { name: tg.name })}
                onClick={() => {
                  setRenameTarget(tg)
                  setRenameValue(tg.name)
                }}
              >
                <Pencil />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                className="size-7"
                aria-label={t("stats.deleteTag", { name: tg.name })}
                onClick={() => setDeleteTarget(tg)}
              >
                <Trash2 />
              </Button>
            </li>
          ))}
        </ul>
      ) : (
        <div className="flex flex-wrap gap-2">
          {tags.map((tg) => (
            <Link key={tg.name} to="/" search={{ tag: tg.name }} className="wisp-chip">
              #{tg.name}
              <span className="text-muted-foreground/70 ml-0.5 tabular-nums">{tg.post_count}</span>
            </Link>
          ))}
        </div>
      )}

      {tags.length === 0 && <p className="text-muted-foreground text-sm">{t("stats.noTags")}</p>}

      <Dialog open={renameTarget !== null} onOpenChange={(v) => !v && setRenameTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("stats.renameTitle")}</DialogTitle>
            <DialogDescription>{t("stats.renameDesc")}</DialogDescription>
          </DialogHeader>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              if (renameTarget && renameValue.trim()) {
                rename.mutate({ id: renameTarget.id, name: renameValue })
              }
            }}
          >
            <Input
              value={renameValue}
              onChange={(e) => setRenameValue(e.target.value)}
              aria-label={t("stats.tagName")}
              autoFocus
            />
            <DialogFooter>
              <Button type="button" variant="ghost" onClick={() => setRenameTarget(null)}>
                {t("common.cancel")}
              </Button>
              <Button type="submit" disabled={!renameValue.trim() || rename.isPending}>
                {t("stats.saveName")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={mergeOpen} onOpenChange={setMergeOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("stats.mergeTitle")}</DialogTitle>
            <DialogDescription>
              {t("stats.mergeDesc", { count: selected.length, name: mergeInto.trim() })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setMergeOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button
              disabled={merge.isPending}
              onClick={() =>
                merge.mutate({
                  from_ids: selected,
                  into: mergeInto.trim(),
                })
              }
            >
              {t("stats.mergeAction")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={deleteTarget !== null} onOpenChange={(v) => !v && setDeleteTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("stats.deleteTagTitle")}</DialogTitle>
            <DialogDescription>
              {t("stats.deleteTagDesc", { name: deleteTarget?.name ?? "" })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDeleteTarget(null)}>
              {t("common.cancel")}
            </Button>
            <Button
              variant="destructive"
              disabled={remove.isPending}
              onClick={() => deleteTarget && remove.mutate(deleteTarget.id)}
            >
              {t("common.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  )
}
