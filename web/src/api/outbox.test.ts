import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

const { kv } = vi.hoisted(() => ({ kv: new Map<string, unknown>() }))

vi.mock("idb-keyval", () => ({
  createStore: () => ({}),
  get: async (key: string) => kv.get(key),
  set: async (key: string, value: unknown) => {
    kv.set(key, value)
  },
  del: async (key: string) => {
    kv.delete(key)
  },
}))

import type { CreatePostInput } from "@/api/posts"

import { flushOutbox, outbox } from "./outbox"

const input: CreatePostInput = { content_md: "离线时写的内容", attachment_ids: [] }

type FakeResponse = { ok: boolean; status: number }

function mockFetch(handler: (call: number) => FakeResponse | Promise<FakeResponse>) {
  let call = 0
  const fn = vi.fn(
    async (_url: string, _init?: RequestInit) => (await handler(++call)) as unknown as Response,
  )
  vi.stubGlobal("fetch", fn)
  return fn
}

beforeEach(() => {
  kv.clear()
  vi.unstubAllGlobals()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe("outbox", () => {
  it("dedupes by idempotency key and supports remove", async () => {
    await outbox.enqueue("k1", input)
    await outbox.enqueue("k1", input)
    await outbox.enqueue("k2", input)

    expect(await outbox.count()).toBe(2)
    expect((await outbox.list()).map((i) => i.key)).toEqual(["k1", "k2"])

    await outbox.remove("k1")
    expect((await outbox.list()).map((i) => i.key)).toEqual(["k2"])
  })
})

describe("flushOutbox", () => {
  it("sends queued posts with idempotency keys and clears them", async () => {
    await outbox.enqueue("k1", input)
    await outbox.enqueue("k2", input)
    const fetchMock = mockFetch(() => ({ ok: true, status: 201 }))

    const result = await flushOutbox()

    expect(result).toEqual({ sent: 2, needsAuth: false, dropped: 0 })
    expect(await outbox.count()).toBe(0)
    const headers = fetchMock.mock.calls[0]?.[1]?.headers as Record<string, string>
    expect(headers["Idempotency-Key"]).toBe("k1")
  })

  it("keeps everything and stops on auth errors", async () => {
    await outbox.enqueue("k1", input)
    await outbox.enqueue("k2", input)
    const fetchMock = mockFetch(() => ({ ok: false, status: 401 }))

    const result = await flushOutbox()

    expect(result).toEqual({ sent: 0, needsAuth: true, dropped: 0 })
    expect(await outbox.count()).toBe(2)
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it("drops only request-level 4xx and continues", async () => {
    await outbox.enqueue("k1", input)
    await outbox.enqueue("k2", input)
    mockFetch((call) => (call === 1 ? { ok: false, status: 400 } : { ok: true, status: 201 }))

    const result = await flushOutbox()

    expect(result).toEqual({ sent: 1, needsAuth: false, dropped: 1 })
    expect(await outbox.count()).toBe(0)
  })

  it("keeps the queue when the server is temporarily unavailable", async () => {
    await outbox.enqueue("k1", input)
    await outbox.enqueue("k2", input)
    const fetchMock = mockFetch(() => ({ ok: false, status: 503 }))

    const result = await flushOutbox()

    expect(result).toEqual({ sent: 0, needsAuth: false, dropped: 0 })
    expect(await outbox.count()).toBe(2)
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it("keeps the queue on network errors", async () => {
    await outbox.enqueue("k1", input)
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("offline")
      }),
    )

    const result = await flushOutbox()

    expect(result).toEqual({ sent: 0, needsAuth: false, dropped: 0 })
    expect(await outbox.count()).toBe(1)
  })
})
