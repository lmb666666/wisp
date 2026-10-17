import { useEffect, type RefObject } from "react"

/**
 * 点击外部或按 Escape 关闭弹层；提供 triggerRef 时把焦点还给触发按钮。
 */
export function useClickOutside<T extends HTMLElement>(
  ref: RefObject<T | null>,
  onDismiss: () => void,
  enabled = true,
  triggerRef?: RefObject<HTMLElement | null>,
) {
  useEffect(() => {
    if (!enabled) return
    const dismiss = () => {
      onDismiss()
      triggerRef?.current?.focus()
    }
    const onPointer = (e: PointerEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) dismiss()
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault()
        dismiss()
      }
    }
    document.addEventListener("pointerdown", onPointer, true)
    document.addEventListener("keydown", onKey)
    return () => {
      document.removeEventListener("pointerdown", onPointer, true)
      document.removeEventListener("keydown", onKey)
    }
  }, [ref, onDismiss, enabled, triggerRef])
}
