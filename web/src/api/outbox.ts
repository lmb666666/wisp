import { createStore, get, set } from "idb-keyval"

import type { CreatePostInput } from "@/api/posts"

export interface OutboxItem {
  key: string
  input: CreatePostInput
  createdAt: number
}

const outboxStore = createStore("wisp-outbox", "outbox")
const LIST_KEY = "items"

// 队列上限：离线积压过多时宁可让发布框保留内容并提示，也不无限堆积。
const MAX_ITEMS = 50

async function read(): Promise<OutboxItem[]> {
  return (await get<OutboxItem[]>(LIST_KEY, outboxStore)) ?? []
}

async function write(items: OutboxItem[]): Promise<void> {
  await set(LIST_KEY, items, outboxStore)
}

// 读改写全部走串行链：flushOutbox 逐条 remove 与并发 enqueue 交错时，
// 不会用旧数组整体覆盖而静默丢条目（跨标签页由服务端幂等键兜底）。
let chain: Promise<unknown> = Promise.resolve()
function serialized<T>(fn: () => Promise<T>): Promise<T> {
  const next = chain.then(fn, fn)
  chain = next.catch(() => {})
  return next
}

export const outbox = {
  /** 入队（网络失败时调用）；队列满时抛错，由调用方提示用户保留内容。 */
  async enqueue(key: string, input: CreatePostInput): Promise<void> {
    return serialized(async () => {
      const items = await read()
      if (items.some((i) => i.key === key)) return
      if (items.length >= MAX_ITEMS) throw new Error("outbox full")
      items.push({ key, input, createdAt: Date.now() })
      await write(items)
    })
  },

  async list(): Promise<OutboxItem[]> {
    return read()
  },

  async count(): Promise<number> {
    return (await read()).length
  },

  async remove(key: string): Promise<void> {
    return serialized(async () => {
      await write((await read()).filter((i) => i.key !== key))
    })
  },

  /** 清空队列（登出时调用：未发内容属于该账号，不能留给共享浏览器的下一位使用者）。 */
  async clear(): Promise<void> {
    return serialized(async () => {
      await write([])
    })
  },
}

let flushing = false

export interface FlushResult {
  sent: number
  /** 队列里还有条目但被 401/403 挡住（会话过期）：等待重新登录，内容不丢。 */
  needsAuth: boolean
  /** 因请求本身错误（4xx，非鉴权）被丢弃的条数。 */
  dropped: number
}

/**
 * 冲刷离线队列：联网恢复 / 定时 / 页面加载时调用。
 * 5xx/408/429 与网络错误保留重试；401/403 保留队列等待重新登录（内容不能丢）；
 * 其余 4xx 请求本身有问题，丢弃。
 */
export async function flushOutbox(): Promise<FlushResult> {
  if (flushing) return { sent: 0, needsAuth: false, dropped: 0 }
  flushing = true
  let sent = 0
  let dropped = 0
  let needsAuth = false
  try {
    let items: OutboxItem[] = []
    try {
      items = await read()
    } catch {
      return { sent: 0, needsAuth: false, dropped: 0 } // IndexedDB 不可用（隐私模式等）：静默跳过
    }
    for (const item of items) {
      try {
        const resp = await fetch("/api/v1/posts", {
          method: "POST",
          credentials: "include",
          headers: { "Content-Type": "application/json", "Idempotency-Key": item.key },
          body: JSON.stringify(item.input),
        })
        if (resp.ok) {
          await outbox.remove(item.key)
          sent++
          continue
        }
        const status = resp.status
        if (status === 401 || status === 403) {
          // 会话过期：内容保留，等重新登录后再冲刷
          needsAuth = true
          break
        }
        const transient = status >= 500 || status === 408 || status === 429
        if (!transient) {
          await outbox.remove(item.key)
          dropped++
          continue
        }
        break // 服务端暂时不可用：保留队列，稍后再试
      } catch {
        // 网络仍不通：留在队列
        break
      }
    }
  } finally {
    flushing = false
  }
  return { sent, needsAuth, dropped }
}
