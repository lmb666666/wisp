import { EditorContent, useEditor, type Editor } from "@tiptap/react"
import StarterKit from "@tiptap/starter-kit"
import Placeholder from "@tiptap/extension-placeholder"
import { useEffect, useRef } from "react"
import { Markdown } from "tiptap-markdown"

export interface MarkdownEditorProps {
  initialContent?: string
  onUpdate?: (markdown: string, editor: Editor) => void
  onReady?: (editor: Editor | null) => void
  onFiles?: (files: File[]) => void
  placeholder?: string
  autoFocus?: boolean
  className?: string
}

const EDITOR_CLASS =
  "min-h-24 w-full px-4 py-3 text-[15px] leading-relaxed outline-none " +
  "[&_p]:my-1 [&_p:first-child]:mt-0 [&_p:last-child]:mb-0 " +
  "[&_h1]:my-2 [&_h1]:text-lg [&_h1]:font-semibold [&_h2]:my-2 [&_h2]:text-base [&_h2]:font-semibold " +
  "[&_pre]:my-2 [&_pre]:rounded-lg [&_pre]:bg-muted [&_pre]:p-3 [&_pre]:text-[13px] " +
  "[&_code]:rounded [&_code]:bg-muted [&_code]:px-1 [&_code]:py-0.5 [&_code]:text-[13px] " +
  "[&_blockquote]:border-border [&_blockquote]:text-muted-foreground [&_blockquote]:my-2 [&_blockquote]:border-l-2 [&_blockquote]:pl-3 " +
  "[&_ul]:my-1 [&_ul]:list-disc [&_ul]:pl-5 [&_ol]:my-1 [&_ol]:list-decimal [&_ol]:pl-5 " +
  "[&_a]:text-primary [&_a]:underline [&_a]:underline-offset-2"

/** TipTap 轻量 Markdown 编辑器：粘贴/拖入图片走 onFiles 拦截。 */
export function MarkdownEditor({
  initialContent,
  onUpdate,
  onReady,
  onFiles,
  placeholder,
  autoFocus,
  className,
}: MarkdownEditorProps) {
  const filesRef = useRef(onFiles)
  useEffect(() => {
    filesRef.current = onFiles
  }, [onFiles])

  const updateRef = useRef(onUpdate)
  useEffect(() => {
    updateRef.current = onUpdate
  }, [onUpdate])

  const editor = useEditor({
    extensions: [
      StarterKit.configure({ heading: { levels: [1, 2, 3] } }),
      Markdown.configure({ breaks: true, html: false }),
      Placeholder.configure({ placeholder: placeholder ?? "" }),
    ],
    content: initialContent ?? "",
    autofocus: autoFocus ?? false,
    editorProps: {
      attributes: {
        class: EDITOR_CLASS + (className ? " " + className : ""),
        "aria-label": placeholder ?? "editor",
      },
      handlePaste: (_view, event) => {
        const files = Array.from(event.clipboardData?.files ?? [])
        if (files.length === 0) return false
        event.preventDefault()
        filesRef.current?.(files)
        return true
      },
      handleDrop: (_view, event) => {
        const files = Array.from(event.dataTransfer?.files ?? [])
        if (files.length === 0) return false
        event.preventDefault()
        filesRef.current?.(files)
        return true
      },
    },
    onUpdate: ({ editor }: { editor: Editor }) => {
      const md = (editor.storage as unknown as { markdown?: { getMarkdown(): string } }).markdown
      updateRef.current?.(md?.getMarkdown() ?? "", editor)
    },
  })

  useEffect(() => {
    onReady?.(editor ?? null)
    return () => onReady?.(null)
  }, [editor, onReady])

  useEffect(() => {
    return () => {
      editor?.destroy()
    }
  }, [editor])

  return <EditorContent editor={editor} />
}
