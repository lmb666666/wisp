import * as React from "react"

import { cn } from "@/lib/utils"

const VARIANTS = {
  default: "border-transparent bg-accent/70 text-accent-foreground",
  secondary: "border-transparent bg-muted text-muted-foreground",
  outline: "bg-transparent text-muted-foreground",
} as const

function Badge({
  className,
  variant = "default",
  ...props
}: React.ComponentProps<"span"> & { variant?: keyof typeof VARIANTS }) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-medium",
        VARIANTS[variant],
        className,
      )}
      {...props}
    />
  )
}

export { Badge }
