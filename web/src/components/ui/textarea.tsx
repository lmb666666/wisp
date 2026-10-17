import * as React from "react"

import { cn } from "@/lib/utils"

function Textarea({ className, ...props }: React.ComponentProps<"textarea">) {
  return (
    <textarea
      className={cn(
        "border-input bg-card flex min-h-16 w-full rounded-xl border px-3 py-2 text-sm shadow-xs transition-colors",
        "placeholder:text-muted-foreground hover:border-ring/60 focus-visible:ring-ring/40 focus-visible:outline-none focus-visible:ring-2",
        "disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    />
  )
}

export { Textarea }
