import { Link, useNavigate, useParams } from "@tanstack/react-router"
import type { Editor } from "@tiptap/react"
import {
  ArrowLeft,
  GitBranch,
  Globe2,
  Loader2,
  MapPin,
  Music4,
  PencilLine,
  Trash2,
  Video,
  X,
} from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { apiError, useMe } from "@/api/auth"
import { api } from "@/api/client"
import { useDeletePost, usePost, useUpdatePost } from "@/api/posts"
import { BlockDialog, type BlockInput, type BlockKind } from "@/components/block-dialog"
import { EditorToolbar } from "@/components/editor-toolbar"
import { MarkdownEditor } from "@/components/markdown-editor"
import { dayLabel, PostCard } from "@/components/post-card"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

/** 说说详情：permalink + 编辑/删除（登录后）。 */
export function PostPage() {
  const { postId } = useParams({ from: "/p/$postId" })
  const { t } = useTranslation()
  const { data: me } = useMe()
  const { data: post, isLoading, isError } = usePost(postId)
  const navigate = useNavigate()
  const del = useDeletePost(postId)
  const [editing, setEditing] = useState(false)
  const [confirming, setConfirming] = useState(false)

  if (isLoading)
    return <p className="text-muted-foreground py-16 text-center text-sm">{t("common.loading")}</p>
  if (isError || !post)
    return (
      <div className="flex flex-col items-center gap-3 py-20 text-center text-sm">
        <p className="text-muted-foreground">{t("post.notFound")}</p>
        <Button asChild variant="outline" size="sm" className="rounded-lg">
          <Link to="/">{t("post.backTimeline")}</Link>
        </Button>
      </div>
    )

  return (
    <div className="wisp-page py-4">
      <div className="mb-3 flex items-center justify-between">
        <Button asChild variant="ghost" size="sm" className="text-muted-foreground -ml-2 gap-1.5">
          <Link to="/">
            <ArrowLeft /> {t("post.back")}
          </Link>
        </Button>
        {me && (
          <div className="flex items-center gap-1">
            <Button
              variant={editing ? "secondary" : "ghost"}
              size="icon"
              aria-label={t("post.edit")}
              title={t("post.edit")}
              onClick={() => setEditing((v) => !v)}
            >
              <PencilLine />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              aria-label={t("post.del")}
              title={t("post.del")}
              onClick={() => setConfirming(true)}
              disabled={del.isPending}
              className="text-muted-foreground hover:text-destructive"
            >
              {del.isPending ? <Loader2 className="animate-spin" /> : <Trash2 />}
            </Button>
          </div>
        )}
      </div>

      <p className="text-muted-foreground mb-2 px-1 text-xs">
        {dayLabel(post.published_at, t)}{" "}
        {new Date(post.published_at * 1000).toLocaleString(undefined, {
          hour: "2-digit",
          minute: "2-digit",
        })}
        {post.edited_at ? ` · ${t("card.edited")}` : ""}
      </p>

      {editing ? (
        <EditPanel post={post} onDone={() => setEditing(false)} />
      ) : (
        <PostCard post={post} variant="detail" />
      )}

      <Dialog open={confirming} onOpenChange={setConfirming}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("post.del")}</DialogTitle>
            <DialogDescription>{t("post.confirmDelete")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setConfirming(false)}>
              {t("common.cancel")}
            </Button>
            <Button
              variant="destructive"
              disabled={del.isPending}
              onClick={() =>
                del.mutate(undefined, {
                  onSuccess: () => {
                    toast.success(t("post.deleted"))
                    navigate({ to: "/" })
                  },
                })
              }
            >
              {del.isPending && <Loader2 className="animate-spin" />} {t("post.del")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function EditPanel({
  post,
  onDone,
}: {
  post: NonNullable<ReturnType<typeof usePost>["data"]>
  onDone: () => void
}) {
  const { t } = useTranslation()
  const [md, setMd] = useState(post.content_md)
  const [visibility, setVisibility] = useState(post.visibility)
  const [publishedAt, setPublishedAt] = useState(toLocalInput(post.published_at))
  const [editor, setEditor] = useState<Editor | null>(null)
  const [blocks, setBlocks] = useState<BlockInput[]>(
    (post.blocks ?? []).map((b) => ({
      type: b.type,
      url: b.url,
      name: b.name,
      lat: b.lat,
      lng: b.lng,
    })),
  )
  const [blockKind, setBlockKind] = useState<BlockKind | null>(null)
  const [blockOpen, setBlockOpen] = useState(false)
  const update = useUpdatePost(post.id)

  const openBlock = async (kind: BlockKind) => {
    if (kind === "music") {
      const cfg = await api<{ url: string }>("/admin/settings/meting").catch(() => ({ url: "" }))
      if (!cfg.url) {
        toast.error(t("publish.metingRequired"))
        return
      }
    }
    setBlockKind(kind)
    setBlockOpen(true)
  }

  const save = () => {
    update.mutate(
      {
        content_md: md,
        visibility,
        blocks,
        published_at: Math.floor(new Date(publishedAt).getTime() / 1000),
      },
      {
        onSuccess: () => {
          toast.success(t("post.saved"))
          onDone()
        },
        onError: (err) => toast.error(apiError(err).message),
      },
    )
  }

  return (
    <div className="wisp-card overflow-hidden">
      <MarkdownEditor
        initialContent={post.content_md}
        onUpdate={setMd}
        onReady={setEditor}
        autoFocus
      />
      <div className="border-border/70 flex flex-wrap items-center gap-1 border-t px-2 py-1.5">
        <EditorToolbar
          editor={editor}
          onMusic={() => void openBlock("music")}
          onGithub={() => void openBlock("github")}
          onWebsite={() => void openBlock("website")}
          onVideo={() => void openBlock("video")}
          onLocation={() => void openBlock("location")}
        />
      </div>
      {blocks.length > 0 && (
        <div className="border-border/70 space-y-2 border-t px-3 py-2">
          {blocks.map((b, i) => (
            <div
              key={i}
              className="border-border flex items-center gap-2 rounded-lg border px-3 py-1.5"
            >
              {b.type === "music" ? (
                <Music4 className="text-muted-foreground size-4" />
              ) : b.type === "github" ? (
                <GitBranch className="text-muted-foreground size-4" />
              ) : b.type === "website" ? (
                <Globe2 className="text-muted-foreground size-4" />
              ) : b.type === "video" ? (
                <Video className="text-muted-foreground size-4" />
              ) : (
                <MapPin className="text-muted-foreground size-4" />
              )}
              <span className="min-w-0 flex-1 truncate text-xs">
                {b.type === "location" ? b.name : b.url}
              </span>
              <button
                type="button"
                aria-label={t("common.remove")}
                className="text-muted-foreground hover:text-foreground"
                onClick={() => setBlocks((prev) => prev.filter((_, j) => j !== i))}
              >
                <X className="size-3.5" />
              </button>
            </div>
          ))}
        </div>
      )}
      <div className="border-border/70 flex flex-wrap items-center gap-2 border-t px-3 py-2">
        <div className="bg-background/70 ring-border/60 flex rounded-[10px] p-0.5 ring-1">
          {(
            [
              ["public", t("publish.visPublic")],
              ["private", t("publish.visPrivate")],
            ] as const
          ).map(([v, label]) => (
            <button
              key={v}
              type="button"
              onClick={() => setVisibility(v)}
              className={
                "rounded-lg px-2.5 py-1 text-xs transition-colors " +
                (visibility === v
                  ? "bg-card text-foreground font-medium shadow-xs"
                  : "text-muted-foreground")
              }
            >
              {label}
            </button>
          ))}
        </div>
        <input
          aria-label={t("post.publishedAt")}
          type="datetime-local"
          value={publishedAt}
          onChange={(e) => setPublishedAt(e.target.value)}
          className="border-input bg-background h-8 rounded-lg border px-2 text-xs"
        />
        <div className="ml-auto flex items-center gap-2">
          <Button variant="ghost" size="sm" onClick={onDone}>
            {t("common.cancel")}
          </Button>
          <Button size="sm" className="rounded-lg" onClick={save} disabled={update.isPending}>
            {update.isPending && <Loader2 className="animate-spin" />} {t("post.save")}
          </Button>
        </div>
      </div>
      <BlockDialog
        kind={blockKind}
        open={blockOpen}
        onOpenChange={setBlockOpen}
        onSubmit={(b) => setBlocks((prev) => (prev.length >= 4 ? prev : [...prev, b]))}
      />
    </div>
  )
}

function toLocalInput(unix: number): string {
  const d = new Date(unix * 1000)
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}
