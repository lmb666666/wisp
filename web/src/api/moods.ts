import type { FetchError } from "ofetch"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { api } from "@/api/client"
import { todayKey } from "@/lib/dates"
import type { Mood, MoodEntry } from "@/lib/moods"

export interface MoodListResp {
  items: MoodEntry[]
}

function isNotFound(err: unknown): boolean {
  return (err as FetchError)?.response?.status === 404
}

/** 区间查询（公开接口）；首页日期头与统计年历共用。 */
export function useMoods(from: string, to: string, enabled = true) {
  return useQuery({
    queryKey: ["moods", from, to],
    queryFn: () => api<MoodListResp>("/moods", { query: { from, to } }),
    enabled,
  })
}

/** 最近一条心情（访客 chip）；没有记录时返回 null。 */
export function useLatestMood(enabled = true) {
  return useQuery({
    queryKey: ["moods", "latest"],
    queryFn: async () => {
      try {
        return await api<MoodEntry>("/moods/latest")
      } catch (err) {
        if (isNotFound(err)) return null
        throw err
      }
    },
    enabled,
    retry: false,
  })
}

/** 今天的心情（作者心情条）。 */
export function useTodayMood() {
  const today = todayKey()
  const query = useMoods(today, today)
  return { today, mood: query.data?.items[0]?.mood, isLoading: query.isLoading }
}

export function useSetMood() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ date, mood }: { date: string; mood: Mood }) =>
      api<MoodEntry>(`/moods/${date}`, { method: "PUT", body: { mood } }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["moods"] }),
  })
}

export function useDeleteMood() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (date: string) => api(`/moods/${date}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["moods"] }),
  })
}
