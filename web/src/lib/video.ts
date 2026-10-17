export interface VideoEmbed {
  embed: string
  provider: "bilibili" | "youtube"
}

const BVID = /(BV[0-9A-Za-z]{10})/
const YT_ID = /^[A-Za-z0-9_-]{6,20}$/

/** 与后端 unfurl.VideoEmbed 保持一致：只接受能确定性解析出 ID 的视频源。 */
export function videoEmbed(raw: string): VideoEmbed | null {
  let u: URL
  try {
    u = new URL(raw.trim())
  } catch {
    return null
  }
  if (u.protocol !== "http:" && u.protocol !== "https:") return null
  const host = u.hostname.toLowerCase()
  if (host === "bilibili.com" || host.endsWith(".bilibili.com")) {
    const m = BVID.exec(u.pathname)
    if (!m) return null
    return {
      embed: `https://player.bilibili.com/player.html?bvid=${m[1]}&autoplay=0`,
      provider: "bilibili",
    }
  }
  if (host === "youtube.com" || host.endsWith(".youtube.com") || host === "youtu.be") {
    let id = ""
    if (host === "youtu.be") id = u.pathname.replace(/^\/+|\/+$/g, "")
    else if (u.searchParams.get("v")) id = u.searchParams.get("v") ?? ""
    else {
      const [kind, vid] = u.pathname.replace(/^\/+|\/+$/g, "").split("/")
      if (kind && vid && ["shorts", "embed", "live"].includes(kind)) id = vid
    }
    if (!YT_ID.test(id)) return null
    return { embed: `https://www.youtube-nocookie.com/embed/${id}`, provider: "youtube" }
  }
  return null
}
