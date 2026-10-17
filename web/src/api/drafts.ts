import { createStore, del, get, set } from "idb-keyval"

import type { Attachment } from "@/api/client"
import type { BlockInput } from "@/components/block-dialog"

const draftStore = createStore("wisp-drafts", "drafts")

/** 发布框草稿：单作者模型，只需一个键。附件存完整对象以便恢复缩略图。 */
export interface Draft {
  content: string
  attachments: Attachment[]
  blocks: BlockInput[]
  updatedAt: number
}

const KEY = "publish"

export async function loadDraft(): Promise<Draft | null> {
  return (await get<Draft>(KEY, draftStore)) ?? null
}

export async function saveDraft(draft: Omit<Draft, "updatedAt">): Promise<void> {
  await set(KEY, { ...draft, updatedAt: Date.now() }, draftStore)
}

export async function clearDraft(): Promise<void> {
  await del(KEY, draftStore)
}
