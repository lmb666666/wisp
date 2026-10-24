import type { FormEvent } from "react"
import { Fingerprint, KeyRound, Loader2, ShieldCheck, UserRound } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  apiError,
  passkeyBeginRegistration,
  passkeyFinishRegistration,
  useDeletePasskey,
  usePasskeys,
  useProfile,
  useUpdateProfile,
} from "@/api/auth"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { passkeyRegister } from "@/lib/webauthn"

/** 设置 · 账号与安全：资料、密码、Passkey。 */
export function PasskeysPage() {
  return (
    <div className="space-y-4">
      <ProfileCard />
      <PasswordCard />
      <PasskeyCard />
    </div>
  )
}

function ProfileCard() {
  const { t } = useTranslation()
  const { data, isLoading, isError } = useProfile()
  if (isError) {
    return (
      <section className="wisp-card p-5">
        <p className="text-muted-foreground text-sm">{t("common.loadFailed")}</p>
      </section>
    )
  }
  if (isLoading || !data) {
    return (
      <section className="wisp-card p-5">
        <p className="text-muted-foreground text-sm">{t("common.loading")}</p>
      </section>
    )
  }
  return <ProfileForm initial={data} />
}

function ProfileForm({ initial }: { initial: NonNullable<ReturnType<typeof useProfile>["data"]> }) {
  const { t } = useTranslation()
  const update = useUpdateProfile()
  const [name, setName] = useState(initial.display_name || "")

  const save = (e: FormEvent) => {
    e.preventDefault()
    update.mutate(
      { display_name: name.trim() },
      {
        onSuccess: () => toast.success(t("settings.saved")),
        onError: (err) => toast.error(apiError(err).message),
      },
    )
  }

  const letter = (initial.display_name || initial.username || "W").slice(0, 1).toUpperCase()

  return (
    <section className="wisp-card p-5">
      <h2 className="flex items-center gap-1.5 wisp-section-title">
        <UserRound className="size-4" /> {t("settings.profileTitle")}
      </h2>
      <form onSubmit={save} className="mt-3 space-y-3">
        <div className="flex items-center gap-3">
          <div className="bg-primary text-primary-foreground flex size-12 shrink-0 items-center justify-center rounded-full text-lg font-semibold shadow-sm">
            {letter}
          </div>
          <div className="text-xs">
            <p className="text-sm font-medium">@{initial.username}</p>
            <p className="text-muted-foreground">
              {t("settings.joinedAt")} {new Date(initial.created_at * 1000).toLocaleDateString()}
            </p>
          </div>
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="display-name">{t("settings.displayName")}</Label>
          <Input
            id="display-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t("settings.displayNamePlaceholder")}
            className="rounded-lg"
          />
        </div>
        <Button type="submit" size="sm" className="rounded-lg" disabled={update.isPending}>
          {update.isPending && <Loader2 className="animate-spin" />} {t("post.save")}
        </Button>
      </form>
    </section>
  )
}

function PasswordCard() {
  const { t } = useTranslation()
  const update = useUpdateProfile()
  const [current, setCurrent] = useState("")
  const [next, setNext] = useState("")

  const save = (e: FormEvent) => {
    e.preventDefault()
    update.mutate(
      { current_password: current, new_password: next },
      {
        onSuccess: () => {
          toast.success(t("settings.passwordChanged"))
          setCurrent("")
          setNext("")
        },
        onError: (err) => toast.error(apiError(err).message),
      },
    )
  }

  return (
    <section className="wisp-card p-5">
      <h2 className="flex items-center gap-1.5 wisp-section-title">
        <ShieldCheck className="size-4" /> {t("settings.passwordTitle")}
      </h2>
      <form onSubmit={save} className="mt-3 grid gap-3 sm:grid-cols-2">
        <div className="space-y-1.5">
          <Label htmlFor="cur-pass">{t("settings.currentPassword")}</Label>
          <Input
            id="cur-pass"
            type="password"
            value={current}
            onChange={(e) => setCurrent(e.target.value)}
            autoComplete="current-password"
            className="rounded-lg"
            required
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="new-pass">{t("settings.newPassword")}</Label>
          <Input
            id="new-pass"
            type="password"
            value={next}
            onChange={(e) => setNext(e.target.value)}
            autoComplete="new-password"
            minLength={8}
            className="rounded-lg"
            required
          />
        </div>
        <div className="sm:col-span-2">
          <Button type="submit" size="sm" className="rounded-lg" disabled={update.isPending || next.length < 8}>
            {update.isPending && <Loader2 className="animate-spin" />} {t("settings.changePassword")}
          </Button>
        </div>
      </form>
    </section>
  )
}

function PasskeyCard() {
  const { t } = useTranslation()
  const { data, isLoading, refetch } = usePasskeys(true)
  const del = useDeletePasskey()
  const [busy, setBusy] = useState(false)

  const register = async () => {
    setBusy(true)
    try {
      const begin = await passkeyBeginRegistration()
      const { session, credential } = await passkeyRegister(begin.session, begin.options)
      await passkeyFinishRegistration(session, credential)
      toast.success(t("settings.passkeyAdded"))
      void refetch()
    } catch (err) {
      toast.error(apiError(err).message || (err instanceof Error ? err.message : t("auth.passkeyFailed")))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="wisp-card p-5">
      <h2 className="flex items-center gap-1.5 wisp-section-title">
        <Fingerprint className="size-4" /> {t("settings.passkeyTitle")}
      </h2>
      <p className="text-muted-foreground mt-1 mb-3 text-xs">{t("settings.accountHint")}</p>
      <Button size="sm" className="rounded-lg" onClick={register} disabled={busy}>
        {busy ? <Loader2 className="animate-spin" /> : <Fingerprint />}
        {busy ? t("settings.waitingDevice") : t("settings.registerPasskey")}
      </Button>
      <div className="mt-3">
        {isLoading && <p className="text-muted-foreground text-sm">{t("common.loading")}</p>}
        <ul className="divide-border divide-y">
          {data?.items.map((k) => (
            <li key={k.id} className="flex items-center justify-between gap-3 py-2.5">
              <div className="min-w-0">
                <p className="flex items-center gap-1.5 truncate text-xs font-medium">
                  <KeyRound className="size-3.5" /> {k.id.slice(0, 20)}…
                </p>
                <p className="text-muted-foreground text-xs">
                  {new Date(k.created_at * 1000).toLocaleDateString()}
                </p>
              </div>
              <Button variant="ghost" size="sm" onClick={() => del.mutate(k.id)} disabled={del.isPending}>
                {t("common.delete")}
              </Button>
            </li>
          ))}
          {(data?.items.length ?? 0) === 0 && !isLoading && (
            <p className="text-muted-foreground py-2 text-sm">{t("settings.noPasskeys")}</p>
          )}
        </ul>
      </div>
    </section>
  )
}
