import { Copy } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { apiError, useCreateToken, useDeleteToken, useTokens } from "@/api/auth"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

/** 设置 · API 令牌：PAT 管理（OpenAPI 的 Bearer 凭证）。 */
export function TokensPage() {
  const { t } = useTranslation()
  const { data, isLoading } = useTokens(true)
  const del = useDeleteToken()

  return (
    <div className="space-y-4">
      <section className="wisp-card p-5">
        <h2 className="wisp-section-title">{t("tokens.title")}</h2>
        <p className="text-muted-foreground mt-1 mb-4 text-xs leading-relaxed">{t("tokens.hint")}</p>
        <CreateTokenForm />
      </section>

      <section className="wisp-card p-5">
        <h3 className="mb-1 text-sm font-medium">{t("settings.existingTokens")}</h3>
        {isLoading && <p className="text-muted-foreground text-sm">{t("common.loading")}</p>}
        <ul className="divide-border divide-y">
          {data?.items.map((tk) => (
            <li key={tk.id} className="flex items-center justify-between gap-3 py-3">
              <div className="min-w-0">
                <p className="truncate text-sm font-medium">{tk.name}</p>
                <p className="text-muted-foreground text-xs">
                  {t("tokens.createdAt")} {new Date(tk.created_at * 1000).toLocaleDateString()}
                  {tk.last_used_at
                    ? ` · ${t("tokens.lastUsed")} ${new Date(tk.last_used_at * 1000).toLocaleString()}`
                    : ` · ${t("tokens.neverUsed")}`}
                </p>
              </div>
              <div className="flex items-center gap-2">
                <Badge variant="outline">{tk.scopes}</Badge>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() =>
                    del.mutate(tk.id, { onSuccess: () => toast.success(t("settings.deleted")) })
                  }
                  disabled={del.isPending}
                >
                  {t("common.delete")}
                </Button>
              </div>
            </li>
          ))}
          {(data?.items.length ?? 0) === 0 && !isLoading && (
            <p className="text-muted-foreground py-3 text-sm">{t("settings.noTokens")}</p>
          )}
        </ul>
      </section>
    </div>
  )
}

function CreateTokenForm() {
  const { t } = useTranslation()
  const [name, setName] = useState("")
  const [scopes, setScopes] = useState("read")
  const create = useCreateToken()
  const [created, setCreated] = useState("")

  const submit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    create.mutate(
      { name: name.trim(), scopes },
      {
        onSuccess: (row) => {
          setCreated(row.token)
          setName("")
          toast.success(t("settings.tokenCreated"))
        },
      },
    )
  }

  return (
    <form onSubmit={submit} className="space-y-3">
      <div className="flex flex-wrap items-end gap-2">
        <div className="min-w-40 flex-1 space-y-1.5">
          <Label htmlFor="token-name">{t("tokens.name")}</Label>
          <Input
            id="token-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t("tokens.namePlaceholder")}
            className="rounded-lg"
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="token-scopes">{t("tokens.scopes")}</Label>
          <Select value={scopes} onValueChange={setScopes}>
            <SelectTrigger id="token-scopes" aria-label={t("tokens.scopes")} className="w-[140px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="read">read</SelectItem>
              <SelectItem value="read,write">read,write</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <Button type="submit" size="sm" className="h-9 rounded-lg" disabled={create.isPending || !name.trim()}>
          {t("tokens.create")}
        </Button>
      </div>
      {create.isError && <p className="text-destructive text-sm">{apiError(create.error).message}</p>}
      {created && (
        <div className="bg-muted/60 rounded-lg p-3">
          <p className="mb-1 text-xs font-medium">{t("tokens.secretOnce")}</p>
          <div className="flex items-start gap-2">
            <code className="block flex-1 break-all text-xs select-all">{created}</code>
            <Button
              size="sm"
              variant="outline"
              className="h-7 rounded-md px-2"
              onClick={() => {
                void navigator.clipboard.writeText(created)
                toast.success(t("settings.copied"))
              }}
            >
              <Copy className="size-3" />
            </Button>
          </div>
        </div>
      )}
    </form>
  )
}
