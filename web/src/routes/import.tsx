import { useState } from "react"
import { useTranslation } from "react-i18next"

import { apiError } from "@/api/client"
import { Upload } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { toast } from "sonner"

const SOURCES = [
  { value: "memos", label: "Memos", hintKey: "memosHint" },
  { value: "moments", label: "Moments", hintKey: "momentsHint" },
  { value: "ech0", label: "Ech0", hintKey: "ech0Hint" },
] as const

/** 导入表单：选择来源 + 上传导出文件，按内容去重、可重复执行。 */
export function ImportSection() {
  const { t } = useTranslation()
  const [source, setSource] = useState<string>("memos")
  const [file, setFile] = useState<File | null>(null)
  const [result, setResult] = useState<string>("")
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!file) return
    setBusy(true)
    setError("")
    setResult("")
    try {
      const fd = new FormData()
      fd.append("source", source)
      fd.append("file", file)
      const resp = await fetch("/api/v1/admin/import", { method: "POST", body: fd, credentials: "include" })
      const data = await resp.json()
      if (!resp.ok) throw data
      const summary =
        t("import.result", {
          total: data.total,
          imported: data.imported,
          skipped: data.skipped,
          failed: data.failed,
        }) + (data.files_imported ? ` · ${t("settings.mediaImported", { n: data.files_imported })}` : "")
      setResult(summary)
      toast.success(summary)
    } catch (err) {
      const msg = apiError(err).message
      setError(msg)
      toast.error(msg)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div>
      <h2 className="wisp-section-title">{t("data.importTitle")}</h2>
      <p className="text-muted-foreground mt-1 mb-4 text-xs leading-relaxed">{t("import.hint")}</p>

      <form onSubmit={submit} className="space-y-4">
        <div className="space-y-2">
          <Label>{t("import.source")}</Label>
          <div className="flex flex-wrap gap-2">
            {SOURCES.map((s) => (
              <button
                key={s.value}
                type="button"
                onClick={() => setSource(s.value)}
                className={
                  "rounded-lg border px-3 py-1.5 text-sm transition-colors " +
                  (source === s.value
                    ? "border-primary/40 bg-primary/10 font-medium"
                    : "border-border hover:bg-accent")
                }
              >
                {s.label}
              </button>
            ))}
          </div>
          <p className="text-muted-foreground text-xs">{t(`import.${SOURCES.find((s) => s.value === source)?.hintKey}`)}</p>
        </div>

        <div className="space-y-1.5">
          <Label>{t("import.file")}</Label>
          <input
            id="import-file"
            type="file"
            accept=".json,.db,.sqlite,.zip"
            className="hidden"
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          />
          <div className="flex items-center gap-2">
            <Button type="button" variant="outline" size="sm" onClick={() => document.getElementById("import-file")?.click()}>
              <Upload className="size-3.5" /> {t("import.chooseFile")}
            </Button>
            <span className="text-muted-foreground truncate text-xs">{file?.name ?? t("import.noFile")}</span>
          </div>
        </div>

        <Button type="submit" size="sm" className="rounded-lg" disabled={busy || !file}>
          {busy ? t("import.importing") : t("import.submit")}
        </Button>

        {result && <p className="text-sm text-green-600 dark:text-green-400">{result}</p>}
        {error && <p className="text-destructive text-sm">{error}</p>}

        <p className="text-muted-foreground border-border/70 border-t pt-3 text-xs leading-relaxed">{t("import.limits")}</p>
      </form>
    </div>
  )
}
