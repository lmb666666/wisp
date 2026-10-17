import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { api, apiError, type Me, type TokenRow } from "@/api/client"

export function useMe() {
  return useQuery({
    queryKey: ["me"],
    queryFn: () => api<Me>("/auth/me"),
    retry: 0,
  })
}

export function useAuthStatus() {
  return useQuery({
    queryKey: ["auth-status"],
    queryFn: () => api<{ initialized: boolean; username?: string }>("/auth/status"),
  })
}

export function useLogin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: { username: string; password: string }) =>
      api<Me>("/auth/login", { method: "POST", body: input }),
    onSuccess: (me) => qc.setQueryData(["me"], me),
  })
}

export function useInit() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: { username: string; password: string; display_name: string }) =>
      api<Me>("/auth/init", { method: "POST", body: input }),
    onSuccess: (me) => qc.setQueryData(["me"], me),
  })
}

export function useLogout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api("/auth/logout", { method: "POST" }),
    onSuccess: () => {
      qc.clear() // 清掉私密时间线/详情等缓存，避免共享浏览器上登出后仍可见
      qc.setQueryData(["me"], null)
    },
  })
}

export function useTokens(enabled: boolean) {
  return useQuery({
    queryKey: ["tokens"],
    queryFn: () => api<{ items: TokenRow[] }>("/auth/tokens"),
    enabled,
  })
}

export function useCreateToken() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: { name: string; scopes: string }) =>
      api<TokenRow & { token: string }>("/auth/tokens", { method: "POST", body: input }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["tokens"] }),
  })
}

export function useDeleteToken() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api(`/auth/tokens/${id}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["tokens"] }),
  })
}

export { apiError }

export interface PasskeyRow {
  id: string
  transports: string
  created_at: number
}

export async function passkeyBeginLogin() {
  return api<{ options: import("@/lib/webauthn").PublicKeyCredentialRequestOptionsJSON; session: string }>(
    "/auth/passkey/login/begin",
    { method: "POST" },
  )
}

export async function passkeyFinishLogin(session: string, credential: Record<string, unknown>) {
  return api<Me>("/auth/passkey/login/finish", {
    method: "POST",
    query: { session },
    body: credential,
  })
}

export async function passkeyBeginRegistration() {
  return api<{ options: import("@/lib/webauthn").PublicKeyCredentialCreationOptionsJSON; session: string }>(
    "/auth/passkey/register/begin",
    { method: "POST" },
  )
}

export async function passkeyFinishRegistration(session: string, credential: Record<string, unknown>) {
  return api<{ ok: boolean }>("/auth/passkey/register/finish", {
    method: "POST",
    query: { session },
    body: credential,
  })
}

export function usePasskeys(enabled: boolean) {
  return useQuery({
    queryKey: ["passkeys"],
    queryFn: () => api<{ items: PasskeyRow[] }>("/auth/passkeys"),
    enabled,
  })
}

export function useDeletePasskey() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api(`/auth/passkeys/${id}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["passkeys"] }),
  })
}

export interface Profile {
  username: string
  display_name: string
  created_at: number
  passkeys: number
}

export function useProfile() {
  return useQuery({
    queryKey: ["profile"],
    queryFn: () => api<Profile>("/auth/profile"),
  })
}

export function useUpdateProfile() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: { display_name?: string; current_password?: string; new_password?: string }) =>
      api<Profile>("/auth/profile", { method: "PATCH", body: input }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["profile"] })
      qc.invalidateQueries({ queryKey: ["me"] })
    },
  })
}
