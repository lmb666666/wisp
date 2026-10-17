import type { Editor } from "@tiptap/react"
import {
  Bold,
  Code2,
  GitBranch,
  Globe2,
  ImagePlus,
  Italic,
  List,
  MapPin,
  Music,
  Quote,
  Video,
} from "lucide-react"
import { useEffect, useReducer } from "react"
import { useTranslation } from "react-i18next"

import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"

interface Props {
  editor: Editor | null
  onImage?: () => void
  onMusic?: () => void
  onGithub?: () => void
  onWebsite?: () => void
  onVideo?: () => void
  onLocation?: () => void
}

/** 编辑器工具栏：激活态可见；支持插入音乐 / GitHub / 位置卡片。 */
export function EditorToolbar({
  editor,
  onImage,
  onMusic,
  onGithub,
  onWebsite,
  onVideo,
  onLocation,
}: Props) {
  const { t } = useTranslation()
  const [, force] = useReducer((x: number) => x + 1, 0)

  useEffect(() => {
    if (!editor) return
    const h = () => force()
    editor.on("transaction", h)
    editor.on("selectionUpdate", h)
    return () => {
      editor.off("transaction", h)
      editor.off("selectionUpdate", h)
    }
  }, [editor])

  const run = (fn: (e: Editor) => void) => {
    if (!editor) return
    fn(editor)
    editor.commands.focus()
  }

  return (
    <TooltipProvider>
      <div className="text-muted-foreground flex flex-wrap items-center gap-x-1 gap-y-1">
        <span className="flex items-center gap-0.5">
          <Item
            active={editor?.isActive("bold")}
            label={t("publish.bold")}
            onClick={() => run((e) => e.chain().focus().toggleBold().run())}
          >
            <Bold className="size-4" />
          </Item>
          <Item
            active={editor?.isActive("italic")}
            label={t("publish.italic")}
            onClick={() => run((e) => e.chain().focus().toggleItalic().run())}
          >
            <Italic className="size-4" />
          </Item>
          <Item
            active={editor?.isActive("code")}
            label={t("publish.code")}
            onClick={() => run((e) => e.chain().focus().toggleCode().run())}
          >
            <Code2 className="size-4" />
          </Item>
          <Item
            active={editor?.isActive("blockquote")}
            label={t("publish.quote")}
            onClick={() => run((e) => e.chain().focus().toggleBlockquote().run())}
          >
            <Quote className="size-4" />
          </Item>
          <Item
            active={editor?.isActive("bulletList")}
            label={t("publish.list")}
            onClick={() => run((e) => e.chain().focus().toggleBulletList().run())}
          >
            <List className="size-4" />
          </Item>
        </span>

        <span className="flex flex-wrap items-center gap-0.5">
          {onImage && (
            <Item label={t("publish.image")} onClick={onImage}>
              <ImagePlus className="size-4" />
            </Item>
          )}
          {onMusic && (
            <Item label={t("publish.music")} onClick={onMusic}>
              <Music className="size-4" />
            </Item>
          )}
          {onGithub && (
            <Item label={t("publish.github")} onClick={onGithub}>
              <GitBranch className="size-4" />
            </Item>
          )}
          {onWebsite && (
            <Item label={t("publish.website")} onClick={onWebsite}>
              <Globe2 className="size-4" />
            </Item>
          )}
          {onVideo && (
            <Item label={t("publish.video")} onClick={onVideo}>
              <Video className="size-4" />
            </Item>
          )}
          {onLocation && (
            <Item label={t("publish.location")} onClick={onLocation}>
              <MapPin className="size-4" />
            </Item>
          )}
        </span>
      </div>
    </TooltipProvider>
  )
}

function Item({
  active,
  label,
  onClick,
  children,
}: {
  active?: boolean
  label: string
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          aria-label={label}
          aria-pressed={active ?? false}
          onClick={onClick}
          className={cn(
            "rounded-lg p-1.5 transition-colors",
            active
              ? "bg-card text-primary ring-border shadow-xs ring-1"
              : "hover:bg-card hover:text-foreground",
          )}
        >
          {children}
        </button>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  )
}
