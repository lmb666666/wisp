import { ofetch, type FetchError } from "ofetch"

/** 访客匿名标识：localStorage 持久化的 UUID。 */
export function getClientID(): string {
  let id = localStorage.getItem("wisp-client-id")
  if (!id) {
    id = crypto.randomUUID()
    localStorage.setItem("wisp-client-id", id)
  }
  return id
}

export const api = ofetch.create({
  baseURL: "/api/v1",
  credentials: "include",
  retry: 0,
  onRequest({ options }) {
    const headers = (options.headers ||= new Headers()) as Headers
    headers.set("X-Client-ID", getClientID())
  },
})

export interface ApiError {
  code: string
  message: string
}

export function apiError(err: unknown): ApiError {
  const e = err as FetchError<{ error?: ApiError }>
  return e?.data?.error ?? { code: "unknown", message: e?.message ?? "request failed" }
}

export interface Attachment {
  id: string
  kind: string
  mime: string
  url: string
  thumb_url?: string
  poster_url?: string
  blurhash?: string
  width: number
  height: number
}

export interface PostBlock {
  type: "music" | "github" | "location" | "website" | "video"
  url?: string
  name?: string
  lat?: number
  lng?: number
  title?: string
  cover?: string
  provider?: string
  audio_url?: string
  embed_url?: string
}

export interface Post {
  id: string
  content_md: string
  content_html: string
  visibility: "public"
  location?: string
  published_at: number
  edited_at?: number
  source: string
  client?: string
  tags: string[]
  attachments: Attachment[]
  blocks: PostBlock[]
}

export interface PostListResp {
  items: Post[]
  has_more: boolean
  next_cursor?: string
}

export interface PostFilters {
  tag?: string
  from?: number
  to?: number
  tags?: string
}

/** tags 参数兼容 JSON 数组（首选）与逗号分隔（旧 URL）。 */
export function splitTagFilter(raw?: string): string[] {
  if (!raw) return []
  const value = raw.trim()
  if (value.startsWith("[")) {
    try {
      const parsed = JSON.parse(value) as unknown
      if (Array.isArray(parsed)) {
        return parsed.filter((x): x is string => typeof x === "string" && x.length > 0)
      }
    } catch {
      return []
    }
  }
  return value.split(",").filter(Boolean)
}

export interface Me {
  username: string
  is_owner: boolean
  via_pat: boolean
  token_name?: string
}

export interface TokenRow {
  id: string
  name: string
  scopes: string
  created_at: number
  last_used_at?: number
}

export async function uploadAttachment(file: File): Promise<Attachment> {
  const fd = new FormData()
  fd.append("file", file)
  return api<Attachment>("/attachments", { method: "POST", body: fd })
}

export async function uploadVideoPoster(id: string, poster: Blob): Promise<string> {
  const fd = new FormData()
  fd.append("file", poster, "poster.jpg")
  return api<{ poster_url: string }>(`/attachments/${id}/poster`, {
    method: "POST",
    body: fd,
  }).then((r) => r.poster_url)
}
