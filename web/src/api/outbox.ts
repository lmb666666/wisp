import { createStore, get, set } from "idb-keyval"

import type { CreatePostInput } from "@/api/posts"

export interface OutboxItem {
  key: string
  input: CreatePostInput
  createdAt: number
}

const outboxStore = createStore("wisp-outbox", "outbox")
const LIST_KEY = "items"

async function read(): Promise<OutboxItem[]> {
  return (await get<OutboxItem[]>(LIST_KEY, outboxStore)) ?? []
}

async function write(items: OutboxItem[]): Promise<void> {
  await set(LIST_KEY, items, outboxStore)
}

export const outbox = {
  /** 入队（网络失败时调用），返回幂等键。 */
  async enqueue(key: string, input: CreatePostInput): Promise<void> {
    const items = await read()
    if (items.some((i) => i.key === key)) return
    items.push({ key, input, createdAt: Date.now() })
    await write(items)
  },

  async list(): Promise<OutboxItem[]> {
    return read()
  },

  async count(): Promise<number> {
    return (await read()).length
  },

  async remove(key: string): Promise<void> {
    await write((await read()).filter((i) => i.key !== key))
  },
}

let flushing = false

/**
 * 冲刷离线队列：联网恢复 / 定时 / 页面加载时调用。
 * 任何「有响应」的结果（含 4xx/5xx）都移除条目——只有纯网络错误才保留重试。
 */
export async function flushOutbox(): Promise<number> {
  if (flushing) return 0
  flushing = true
  let sent = 0
  try {
    let items: OutboxItem[] = []
    try {
      items = await read()
    } catch {
      return 0 // IndexedDB 不可用（隐私模式等）：静默跳过
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
        const transient = status >= 500 || status === 408 || status === 429
        if (!transient) {
          // 4xx：请求本身有问题，重试也没用
          await outbox.remove(item.key)
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
  return sent
}
