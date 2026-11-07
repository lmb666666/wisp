import { splitTagFilter, type PostFilters } from "@/api/client"

/** 当前激活的筛选条件数量（日期区间算一个 + 去重后的标签数）。 */
export function activeFilterCount(f: PostFilters): number {
  let n = 0
  if (f.from || f.to) n++
  const tags = new Set(splitTagFilter(f.tags))
  if (f.tag) tags.add(f.tag)
  return n + tags.size
}
