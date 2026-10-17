import { Toaster as Sonner } from "sonner"

/** 右下角 toast（shadcn/sonner）。 */
export function Toaster() {
  return (
    <Sonner
      position="bottom-right"
      richColors
      closeButton
      offset={16}
      toastOptions={{ className: "rounded-xl text-sm" }}
    />
  )
}
