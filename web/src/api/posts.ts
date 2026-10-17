import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import type { InfiniteData } from "@tanstack/react-query"

import { api, type Post, type PostFilters, type PostListResp } from "@/api/client"

const PAGE_SIZE = 20

/** 只把新帖插入未筛选的时间线缓存，避免它出现在不匹配的筛选结果里。 */
function updateUnfilteredPostQueries(
  qc: ReturnType<typeof useQueryClient>,
  update: (data: InfiniteData<PostListResp>) => InfiniteData<PostListResp>,
) {
  for (const [key, data] of qc.getQueriesData<InfiniteData<PostListResp>>({
    queryKey: ["posts"],
  })) {
    if (!data) continue
    const filters = key.slice(1)
    if (filters.some((value) => Boolean(value))) continue
    qc.setQueryData(key, update(data))
  }
}

export function usePosts(filters: PostFilters = {}) {
  const { tag, from, to, vis, tags } = filters
  return useInfiniteQuery({
    queryKey: ["posts", tag ?? "", from ?? 0, to ?? 0, vis ?? "", tags ?? ""],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) =>
      api<PostListResp>("/posts", {
        params: {
          limit: PAGE_SIZE,
          before: pageParam,
          tag: tag || undefined,
          from: from || undefined,
          to: to || undefined,
          vis: vis || undefined,
          tags: tags || undefined,
        },
      }),
    getNextPageParam: (last) => (last.has_more ? last.next_cursor : undefined),
  })
}

export function usePost(id: string) {
  return useQuery({
    queryKey: ["post", id],
    queryFn: () => api<Post>(`/posts/${id}`),
    retry: 0,
  })
}

export function useAddReaction(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (emoji: string) =>
      api(`/posts/${id}/reactions`, { method: "POST", body: { emoji } }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["post", id] })
      qc.invalidateQueries({ queryKey: ["posts"] })
    },
  })
}

export function useRemoveReaction(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (emoji: string) =>
      api(`/posts/${id}/reactions?emoji=${encodeURIComponent(emoji)}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["post", id] })
      qc.invalidateQueries({ queryKey: ["posts"] })
    },
  })
}

export function useSearch(q: string) {
  return useQuery({
    queryKey: ["search", q],
    queryFn: () => api<{ items: Post[] }>(`/search?q=${encodeURIComponent(q)}`),
    enabled: q.trim().length > 0,
  })
}

export interface CreatePostInput {
  content_md: string
  visibility: string
  published_at?: number
  attachment_ids: string[]
  client?: string
  location?: string
  blocks?: {
    type: "music" | "github" | "location" | "website" | "video"
    url?: string
    name?: string
    lat?: number
    lng?: number
  }[]
}

export function useCreatePost() {
  const qc = useQueryClient()
  return useMutation({
    // key：幂等键（离线重试时服务端按 Idempotency-Key 去重）
    mutationFn: (input: CreatePostInput & { key?: string }) =>
      api<Post>("/posts", {
        method: "POST",
        body: (() => {
          const { key, ...body } = input
          void key
          return body
        })(),
        headers: input.key ? { "Idempotency-Key": input.key } : undefined,
      }),
    // 乐观发送：3 秒发布的关键——先把帖子塞进时间线，失败再回滚
    onMutate: async (input) => {
      await qc.cancelQueries({ queryKey: ["posts"] })
      const optimistic: Post = {
        id: `optimistic-${Date.now()}`,
        content_md: input.content_md,
        content_html: "<p>" + input.content_md.replace(/\n/g, "<br/>") + "</p>",
        visibility: (input.visibility as Post["visibility"]) ?? "public",
        location: input.location,
        published_at: input.published_at ?? Math.floor(Date.now() / 1000),
        source: "web",
        tags: [],
        attachments: [],
        reactions: [],
        blocks: [],
      }
      updateUnfilteredPostQueries(qc, (data) => {
        const pages = data.pages.map((p, i) =>
          i === 0 ? { ...p, items: [optimistic, ...p.items] } : p,
        )
        return { ...data, pages }
      })
      return { optimistic }
    },
    onError: (_err, _input, ctx) => {
      if (!ctx?.optimistic) return
      qc.setQueriesData<InfiniteData<PostListResp>>({ queryKey: ["posts"] }, (data) => {
        if (!data) return data
        const pages = data.pages.map((p, i) =>
          i === 0 ? { ...p, items: p.items.filter((x) => x.id !== ctx.optimistic!.id) } : p,
        )
        return { ...data, pages }
      })
    },
    onSuccess: (real) => {
      // 用服务端真实数据替换乐观占位（清掉占位、保留其余真实帖）
      updateUnfilteredPostQueries(qc, (data) => {
        const pages = data.pages.map((p, i) => {
          if (i !== 0) return p
          const withoutOptimistic = p.items.filter((x) => !x.id.startsWith("optimistic-"))
          return { ...p, items: [real, ...withoutOptimistic] }
        })
        return { ...data, pages }
      })
      void qc.invalidateQueries({ queryKey: ["tags"] })
      void qc.invalidateQueries({ queryKey: ["search"] })
      void qc.invalidateQueries({ queryKey: ["overview"] })
      // 服务端异步抓取链接/音乐卡片；冷启动的 Meting 可能较慢，补两次刷新
      for (const delay of [1600, 9000]) {
        window.setTimeout(() => {
          void qc.invalidateQueries({ queryKey: ["posts"] })
        }, delay)
      }
    },
  })
}

export function useUpdatePost(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: Record<string, unknown>) =>
      api<Post>(`/posts/${id}`, { method: "PATCH", body: input }),
    onSuccess: (post) => {
      qc.setQueryData(["post", id], post)
      qc.invalidateQueries({ queryKey: ["posts"] })
      qc.invalidateQueries({ queryKey: ["tags"] })
      qc.invalidateQueries({ queryKey: ["search"] })
      qc.invalidateQueries({ queryKey: ["overview"] })
    },
  })
}

export function useDeletePost(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api(`/posts/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.removeQueries({ queryKey: ["post", id] })
      qc.invalidateQueries({ queryKey: ["posts"] })
      qc.invalidateQueries({ queryKey: ["tags"] })
      qc.invalidateQueries({ queryKey: ["search"] })
      qc.invalidateQueries({ queryKey: ["overview"] })
    },
  })
}
