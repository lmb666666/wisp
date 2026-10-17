import imageCompression from "browser-image-compression"
import type { Editor } from "@tiptap/react"
import { useQuery } from "@tanstack/react-query"
import { GitBranch, Globe2, Loader2, MapPin, Music4, Send, Video, X } from "lucide-react"
import { Suspense, lazy, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { api, apiError, uploadAttachment, uploadVideoPoster, type Attachment } from "@/api/client"
import { clearDraft, loadDraft, saveDraft } from "@/api/drafts"
import { outbox } from "@/api/outbox"
import { useCreatePost } from "@/api/posts"
import { BlockDialog, type BlockInput, type BlockKind } from "@/components/block-dialog"
import { EditorToolbar } from "@/components/editor-toolbar"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"

const MarkdownEditor = lazy(() =>
  import("@/components/markdown-editor").then((m) => ({ default: m.MarkdownEditor })),
)

/** 截取视频首帧为 JPEG（浏览器 canvas，保持服务端纯 Go）。 */
function capturePoster(file: File): Promise<Blob | null> {
  return new Promise((resolve) => {
    const video = document.createElement("video")
    video.preload = "metadata"
    video.muted = true
    video.src = URL.createObjectURL(file)
    const done = (b: Blob | null) => {
      URL.revokeObjectURL(video.src)
      resolve(b)
    }
    video.onloadeddata = () => {
      video.currentTime = Math.min(0.5, (video.duration || 1) / 2)
    }
    video.onseeked = () => {
      const canvas = document.createElement("canvas")
      canvas.width = video.videoWidth
      canvas.height = video.videoHeight
      canvas.getContext("2d")?.drawImage(video, 0, 0)
      canvas.toBlob((b) => done(b), "image/jpeg", 0.85)
    }
    video.onerror = () => done(null)
  })
}

const VISIBILITIES = [
  { value: "public", key: "visPublic" },
  { value: "private", key: "visPrivate" },
] as const

/** 发布框：常驻时间线顶部，3 秒发出。 */
export function PublishBox({ initialShare }: { initialShare?: string }) {
  const [ready, setReady] = useState(false)
  const [draft, setDraft] = useState<{
    content: string
    attachments: Attachment[]
    blocks: BlockInput[]
  } | null>(null)
  const [sharedText, setSharedText] = useState(initialShare ?? "")
  const [resetKey, setResetKey] = useState(0)

  useEffect(() => {
    let cancelled = false
    const init: Promise<void> = initialShare
      ? Promise.resolve()
      : loadDraft()
          .catch(() => null) // IndexedDB 不可用时不影响发布
          .then((d) => {
            if (d?.content || d?.attachments?.length || d?.blocks?.length) {
              setDraft({
                content: d.content,
                attachments: d.attachments ?? [],
                blocks: d.blocks ?? [],
              })
            }
          })
    init.then(() => {
      if (!cancelled) setReady(true)
    })
    return () => {
      cancelled = true
    }
  }, [initialShare])

  if (!ready) return <Skeleton className="my-4 h-40" />

  return (
    <PublishBoxInner
      key={`${resetKey}-${sharedText}`}
      initial={sharedText || draft?.content || ""}
      initialAttachments={draft?.attachments ?? []}
      initialBlocks={draft?.blocks ?? []}
      onPublished={() => {
        setDraft(null)
        setSharedText("")
        setResetKey((n) => n + 1)
      }}
    />
  )
}

function PublishBoxInner({
  initial,
  initialAttachments,
  initialBlocks,
  onPublished,
}: {
  initial: string
  initialAttachments: Attachment[]
  initialBlocks: BlockInput[]
  onPublished: () => void
}) {
  const [markdown, setMarkdown] = useState(initial)
  const [visibility, setVisibility] = useState<string>("public")
  const [attachments, setAttachments] = useState<Attachment[]>(initialAttachments)
  const [blocks, setBlocks] = useState<BlockInput[]>(initialBlocks)
  const [blockKind, setBlockKind] = useState<BlockKind | null>(null)
  const [blockOpen, setBlockOpen] = useState(false)
  const [uploading, setUploading] = useState(0)
  const [tagHit, setTagHit] = useState<{ start: number; query: string } | null>(null)
  const [editor, setEditor] = useState<Editor | null>(null)
  const editorRef = useRef<Editor | null>(null)
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const firstRender = useRef(true)
  const { t } = useTranslation()
  const create = useCreatePost()

  const { data: tagList } = useQuery({
    queryKey: ["tags"],
    queryFn: () => api<{ items: { name: string; post_count: number }[] }>("/tags"),
    staleTime: 60_000,
  })
  const suggestions = (tagList?.items ?? [])
    .map((tg) => tg.name)
    .filter(
      (n) => tagHit && n.toLowerCase().startsWith(tagHit.query.toLowerCase()) && n !== tagHit.query,
    )
    .slice(0, 6)

  useEffect(() => {
    // 初次挂载不写回：避免打开 /share 或恢复草稿时覆盖已有草稿
    if (firstRender.current) {
      firstRender.current = false
      return
    }
    const timer = setTimeout(() => saveDraft({ content: markdown, attachments, blocks }), 400)
    return () => clearTimeout(timer)
  }, [markdown, attachments, blocks])

  const handleFiles = async (files: File[]) => {
    setUploading((n) => n + files.length)
    for (const file of files) {
      try {
        let payload: File = file
        if (file.type.startsWith("image/") && file.type !== "image/gif") {
          payload = await imageCompression(file, {
            maxSizeMB: 4,
            maxWidthOrHeight: 2560,
            useWebWorker: true,
          })
        }
        const att = await uploadAttachment(payload)
        if (att.kind === "video") {
          const poster = await capturePoster(payload)
          if (poster)
            att.poster_url = await uploadVideoPoster(att.id, poster).catch(() => att.poster_url)
        }
        setAttachments((prev) => [...prev, att])
      } catch (err) {
        toast.error(apiError(err).message)
      } finally {
        setUploading((n) => n - 1)
      }
    }
  }

  const handleUpdate = (md: string, ed: Editor | null) => {
    setMarkdown(md)
    editorRef.current = ed
    if (!ed) {
      setTagHit(null)
      return
    }
    const from = ed.state.selection.from
    const before = ed.state.doc.textBetween(Math.max(0, from - 32), from, "\n", "\n")
    const m = /(?:^|\s)#([^\s#]{0,24})$/.exec(before)
    const query = m?.[1] ?? ""
    setTagHit(query ? { start: from - query.length, query } : null)
  }

  const applyTag = (name: string) => {
    const ed = editorRef.current
    if (!ed || !tagHit) return
    ed.chain()
      .focus()
      .insertContentAt({ from: tagHit.start, to: tagHit.start + tagHit.query.length }, name + " ")
      .run()
    setTagHit(null)
  }

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

  const addBlock = (b: BlockInput) => setBlocks((prev) => (prev.length >= 4 ? prev : [...prev, b]))

  const publish = () => {
    const content = markdown.trim()
    if (!content && attachments.length === 0 && blocks.length === 0) return
    const key = crypto.randomUUID()
    const input = {
      content_md: content,
      visibility,
      attachment_ids: attachments.map((a) => a.id),
      blocks,
      client: "web",
    }
    create.mutate(
      { ...input, key },
      {
        onSuccess: () => {
          toast.success(t("publish.published"))
          setMarkdown("")
          setAttachments([])
          setBlocks([])
          clearDraft()
          onPublished()
        },
        onError: (err) => {
          const status = (err as { statusCode?: number }).statusCode
          if (!status) {
            outbox.enqueue(key, input).then(() => toast.info(t("publish.offlineQueued")))
            setMarkdown("")
            setAttachments([])
            setBlocks([])
            clearDraft()
            onPublished()
          } else {
            toast.error(apiError(err).message)
          }
        },
      },
    )
  }

  const canPublish =
    (markdown.trim().length > 0 || attachments.length > 0 || blocks.length > 0) && uploading === 0
  const chars = [...markdown].length

  return (
    <div className="wisp-card my-4 overflow-hidden">
      <Suspense fallback={<Skeleton className="m-3 h-28" />}>
        <MarkdownEditor
          initialContent={initial}
          onUpdate={handleUpdate}
          onReady={(ed) => {
            editorRef.current = ed
            setEditor(ed)
          }}
          onFiles={handleFiles}
          placeholder={t("publish.placeholder")}
        />
      </Suspense>

      {tagHit && suggestions.length > 0 && (
        <div className="wisp-pop text-muted-foreground mx-4 mb-2 flex flex-wrap items-center gap-1.5 text-xs">
          <span>{t("publish.tagSuggestions")}</span>
          {suggestions.map((name) => (
            <button key={name} type="button" className="wisp-chip" onClick={() => applyTag(name)}>
              #{name}
            </button>
          ))}
        </div>
      )}

      {blocks.length > 0 && (
        <div className="mx-4 mb-3 space-y-2">
          {blocks.map((b, i) => (
            <div
              key={i}
              className="border-border flex items-center gap-2 rounded-xl border px-3 py-2"
            >
              {b.type === "music" ? (
                <Music4 className="text-muted-foreground size-4 shrink-0" />
              ) : b.type === "github" ? (
                <GitBranch className="text-muted-foreground size-4 shrink-0" />
              ) : b.type === "website" ? (
                <Globe2 className="text-muted-foreground size-4 shrink-0" />
              ) : b.type === "video" ? (
                <Video className="text-muted-foreground size-4 shrink-0" />
              ) : (
                <MapPin className="text-muted-foreground size-4 shrink-0" />
              )}
              <div className="min-w-0 flex-1">
                <p className="truncate text-xs font-medium">
                  {b.type === "location"
                    ? b.name
                    : b.type === "music"
                      ? t("publish.music")
                      : b.type === "github"
                        ? t("publish.github")
                        : b.type === "website"
                          ? t("publish.website")
                          : t("publish.video")}
                </p>
                <p className="text-muted-foreground truncate text-[11px]">
                  {b.type === "location"
                    ? b.lat !== undefined && b.lng !== undefined
                      ? `${b.lat.toFixed(4)}, ${b.lng.toFixed(4)}`
                      : t("blocks.locationLabel")
                    : b.url}
                </p>
              </div>
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

      {attachments.length > 0 && (
        <div className="mx-4 mb-3 grid grid-cols-3 gap-2">
          {attachments.map((a) => (
            <div
              key={a.id}
              className="wisp-pop bg-muted group relative aspect-square overflow-hidden rounded-xl"
            >
              {a.kind === "image" ? (
                <img src={a.thumb_url || a.url} alt="" className="h-full w-full object-cover" />
              ) : (
                <div className="text-muted-foreground flex h-full items-center justify-center text-xs">
                  {a.kind.toUpperCase()}
                </div>
              )}
              <button
                type="button"
                aria-label={t("common.remove")}
                onClick={() => setAttachments((prev) => prev.filter((x) => x.id !== a.id))}
                className="bg-background/90 text-foreground absolute top-1 right-1 rounded-full p-1 opacity-100 shadow-sm transition-opacity focus-visible:opacity-100 sm:opacity-0 sm:group-hover:opacity-100"
              >
                <X className="size-3" />
              </button>
            </div>
          ))}
        </div>
      )}

      <div className="bg-muted/50 border-border/70 flex flex-wrap items-center gap-1 gap-y-1.5 border-t px-2 py-1.5">
        <EditorToolbar
          editor={editor}
          onImage={() => fileInputRef.current?.click()}
          onMusic={() => void openBlock("music")}
          onGithub={() => void openBlock("github")}
          onWebsite={() => void openBlock("website")}
          onVideo={() => void openBlock("video")}
          onLocation={() => void openBlock("location")}
        />
        <input
          ref={fileInputRef}
          type="file"
          accept="image/*,video/*,audio/*"
          multiple
          className="hidden"
          onChange={(e) => {
            const files = Array.from(e.target.files ?? [])
            if (files.length) void handleFiles(files)
            e.target.value = ""
          }}
        />

        <div className="ml-auto flex shrink-0 items-center gap-2">
          <div className="bg-background/70 ring-border/60 flex rounded-[10px] p-0.5 ring-1">
            {VISIBILITIES.map((v) => (
              <button
                key={v.value}
                type="button"
                onClick={() => setVisibility(v.value)}
                className={
                  "rounded-md px-2 py-1 text-xs transition-colors " +
                  (visibility === v.value
                    ? "bg-background text-foreground font-medium shadow-sm"
                    : "text-muted-foreground hover:text-foreground")
                }
              >
                {t(`publish.${v.key}`)}
              </button>
            ))}
          </div>
          {chars > 0 && (
            <span className="text-muted-foreground hidden text-[11px] tabular-nums sm:inline">
              {chars} {t("publish.chars")}
            </span>
          )}
          <Button
            size="sm"
            onClick={publish}
            disabled={create.isPending || !canPublish}
            className="h-8 shrink-0 rounded-lg px-3"
          >
            {create.isPending ? (
              <Loader2 className="animate-spin" />
            ) : (
              <Send className="size-3.5" />
            )}
            {t("publish.submit")}
          </Button>
        </div>
      </div>
      <BlockDialog
        kind={blockKind}
        open={blockOpen}
        onOpenChange={setBlockOpen}
        onSubmit={addBlock}
      />
    </div>
  )
}
