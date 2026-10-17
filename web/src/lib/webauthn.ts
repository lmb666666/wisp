import i18next from "i18next"

/** WebAuthn 浏览器侧封装：base64url 编解码 + 注册/发现式登录。 */

export interface CredentialCreationOptionsJSON {
  options: PublicKeyCredentialCreationOptionsJSON
  session: string
}

export interface PublicKeyCredentialCreationOptionsJSON {
  publicKey: {
    rp: { id: string; name: string }
    user: { id: string; name: string; displayName: string }
    challenge: string
    pubKeyCredParams: { type: string; alg: number }[]
    timeout?: number
    excludeCredentials?: { id: string; type: string }[]
    authenticatorSelection: {
      residentKey: string
      userVerification: string
    }
    attestation: string
  }
}

export interface PublicKeyCredentialRequestOptionsJSON {
  publicKey: {
    challenge: string
    rpId: string
    timeout?: number
    userVerification: string
  }
}

function bufToB64url(buf: ArrayBuffer): string {
  const bytes = new Uint8Array(buf)
  let bin = ""
  for (const b of bytes) bin += String.fromCharCode(b)
  return btoa(bin).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "")
}

function b64urlToBuf(s: string): ArrayBuffer {
  const pad = s.length % 4 === 0 ? "" : "=".repeat(4 - (s.length % 4))
  const bin = atob(s.replace(/-/g, "+").replace(/_/g, "/") + pad)
  const bytes = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i)
  return bytes.buffer
}

type CreateCred = { rawId: ArrayBuffer; response: AuthenticatorAttestationResponse }
type GetCred = { rawId: ArrayBuffer; response: AuthenticatorAssertionResponse }

function credToJSON(cred: CreateCred | GetCred, type: "create" | "get"): Record<string, unknown> {
  const base = {
    id: bufToB64url(cred.rawId),
    rawId: bufToB64url(cred.rawId),
    type: "public-key",
  }
  if (type === "create") {
    const res = cred.response as AuthenticatorAttestationResponse
    return {
      ...base,
      response: {
        attestationObject: bufToB64url(res.attestationObject),
        clientDataJSON: bufToB64url(res.clientDataJSON),
        transports: res.getTransports?.() ?? [],
      },
    }
  }
  const res = cred.response as AuthenticatorAssertionResponse
  return {
    ...base,
    response: {
      authenticatorData: bufToB64url(res.authenticatorData),
      clientDataJSON: bufToB64url(res.clientDataJSON),
      signature: bufToB64url(res.signature),
      userHandle: res.userHandle ? bufToB64url(res.userHandle) : null,
    },
  }
}

/** 注册新 Passkey（页面已登录）。返回仪式 ID 与凭证 JSON。 */
export async function passkeyRegister(session: string, optionsJSON: PublicKeyCredentialCreationOptionsJSON) {
  const o = optionsJSON.publicKey
  const cred = (await navigator.credentials.create({
    publicKey: {
      challenge: b64urlToBuf(o.challenge),
      rp: o.rp,
      user: { id: b64urlToBuf(o.user.id), name: o.user.name, displayName: o.user.displayName },
      pubKeyCredParams: o.pubKeyCredParams as PublicKeyCredentialParameters[],
      excludeCredentials: (o.excludeCredentials ?? []).map((c) => ({
        id: b64urlToBuf(c.id),
        type: "public-key" as const,
      })),
      authenticatorSelection: {
        residentKey: o.authenticatorSelection.residentKey as ResidentKeyRequirement,
        userVerification: o.authenticatorSelection.userVerification as UserVerificationRequirement,
      },
      attestation: o.attestation as AttestationConveyancePreference,
    },
  })) as CreateCred | null
  if (!cred) throw new Error(i18next.t("auth.passkeyFailed"))
  return {
    session,
    credential: credToJSON(cred, "create"),
  }
}

/** 发现式登录（无需用户名）。返回仪式 ID 与断言 JSON。 */
export async function passkeyLogin(session: string, optionsJSON: PublicKeyCredentialRequestOptionsJSON) {
  const o = optionsJSON.publicKey
  const cred = (await navigator.credentials.get({
    publicKey: {
      challenge: b64urlToBuf(o.challenge),
      rpId: o.rpId,
      userVerification: o.userVerification as UserVerificationRequirement,
    },
  })) as GetCred | null
  if (!cred) throw new Error(i18next.t("auth.passkeyFailed"))
  return {
    session,
    credential: credToJSON(cred, "get"),
  }
}
