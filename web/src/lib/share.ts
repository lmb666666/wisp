export type ShareResult = "shared" | "copied" | "dismissed" | "failed"

async function copyText(text: string): Promise<boolean> {
  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // 权限被拒时退回 execCommand
    }
  }
  const el = document.createElement("textarea")
  el.value = text
  el.setAttribute("readonly", "")
  el.style.position = "fixed"
  el.style.opacity = "0"
  document.body.appendChild(el)
  el.select()
  let ok = false
  try {
    ok = document.execCommand("copy")
  } catch {
    // 浏览器禁用 execCommand 时保持失败
  }
  el.remove()
  return ok
}

/** 优先系统分享；否则复制链接。用户取消系统分享时静默返回 dismissed。 */
export async function sharePost(input: {
  title: string
  text: string
  url: string
}): Promise<ShareResult> {
  if (typeof navigator.share === "function") {
    try {
      await navigator.share(input)
      return "shared"
    } catch (err) {
      if (err instanceof DOMException && err.name === "AbortError") return "dismissed"
    }
  }
  return (await copyText(input.url)) ? "copied" : "failed"
}
