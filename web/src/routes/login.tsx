import { useNavigate } from "@tanstack/react-router"
import { useQueryClient } from "@tanstack/react-query"
import { Fingerprint, Loader2 } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"

import { apiError, passkeyBeginLogin, passkeyFinishLogin, useAuthStatus, useInit, useLogin } from "@/api/auth"
import { AppLogo } from "@/components/app-logo"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { passkeyLogin } from "@/lib/webauthn"

/** 登录页：未初始化时展示首启向导（创建 owner），否则登录表单。 */
export function Login() {
  const { t } = useTranslation()
  const { data: status, isLoading } = useAuthStatus()
  if (isLoading) return <p className="text-muted-foreground py-16 text-center text-sm">{t("common.loading")}</p>

  return status?.initialized ? <LoginForm /> : <InitForm />
}

function AuthShell({ title, hint, children }: { title: string; hint?: string; children: React.ReactNode }) {
  return (
    <div className="wisp-page wisp-auth-bg flex min-h-[calc(100dvh-8rem)] items-center justify-center py-8">
      <div className="wisp-card w-full max-w-sm p-7" style={{ boxShadow: "var(--shadow-lift)" }}>
        <div className="mb-5 flex flex-col items-center gap-3 text-center">
          <AppLogo className="size-11" />
          <div>
            <h1 className="font-display text-2xl font-semibold tracking-tight">{title}</h1>
            {hint && <p className="text-muted-foreground mt-1 text-sm">{hint}</p>}
          </div>
        </div>
        {children}
      </div>
    </div>
  )
}

function PasskeyButton() {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)

  const go = async () => {
    setBusy(true)
    setError("")
    try {
      const begin = await passkeyBeginLogin()
      const { credential } = await passkeyLogin(begin.session, begin.options)
      const me = await passkeyFinishLogin(begin.session, credential)
      qc.setQueryData(["me"], me)
      navigate({ to: "/" })
    } catch (err) {
      setError(err instanceof Error ? err.message : t("auth.passkeyFailed"))
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <Button type="button" variant="outline" className="h-10 w-full rounded-xl" onClick={go} disabled={busy}>
        {busy ? <Loader2 className="animate-spin" /> : <Fingerprint />} {t("auth.passkey")}
      </Button>
      {error && <p className="text-destructive text-center text-sm">{error}</p>}
    </>
  )
}

function LoginForm() {
  const { t } = useTranslation()
  const login = useLogin()
  const navigate = useNavigate()
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")

  const submit = (e: React.FormEvent) => {
    e.preventDefault()
    login.mutate({ username, password }, { onSuccess: () => navigate({ to: "/" }) })
  }

  return (
    <AuthShell title={t("auth.loginTitle")}>
      <form onSubmit={submit} className="space-y-4">
        <div className="space-y-1.5">
          <Label htmlFor="username">{t("auth.username")}</Label>
          <Input id="username" value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" required />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="password">{t("auth.password")}</Label>
          <Input
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            required
          />
        </div>
        {login.isError && <p className="text-destructive text-sm">{apiError(login.error).message}</p>}
        <Button type="submit" className="h-10 w-full rounded-xl" disabled={login.isPending}>
          {login.isPending && <Loader2 className="animate-spin" />}
          {t("auth.submit")}
        </Button>
        <div className="text-muted-foreground flex items-center gap-3 text-xs">
          <div className="bg-border h-px flex-1" /> {t("common.or")} <div className="bg-border h-px flex-1" />
        </div>
        <PasskeyButton />
      </form>
    </AuthShell>
  )
}

function InitForm() {
  const { t } = useTranslation()
  const init = useInit()
  const navigate = useNavigate()
  const [username, setUsername] = useState("")
  const [displayName, setDisplayName] = useState("")
  const [password, setPassword] = useState("")

  const submit = (e: React.FormEvent) => {
    e.preventDefault()
    init.mutate({ username, password, display_name: displayName }, { onSuccess: () => navigate({ to: "/" }) })
  }

  return (
    <AuthShell title={t("auth.initTitle")} hint={t("auth.initHint")}>
      <form onSubmit={submit} className="space-y-4">
        <div className="space-y-1.5">
          <Label htmlFor="i-username">{t("auth.username")}</Label>
          <Input id="i-username" value={username} onChange={(e) => setUsername(e.target.value)} required />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="i-display">{t("auth.displayName")}</Label>
          <Input id="i-display" value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="i-password">{t("auth.passwordInit")}</Label>
          <Input
            id="i-password"
            type="password"
            minLength={8}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="new-password"
            required
          />
        </div>
        {init.isError && <p className="text-destructive text-sm">{apiError(init.error).message}</p>}
        <Button type="submit" className="h-10 w-full rounded-xl" disabled={init.isPending}>
          {init.isPending && <Loader2 className="animate-spin" />}
          {t("auth.initSubmit")}
        </Button>
      </form>
    </AuthShell>
  )
}
