import * as React from "react"

import { cn } from "@/lib/utils"

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      className={cn(
        "border-input bg-card flex h-9 w-full appearance-none rounded-[10px] border px-3 py-1 text-[16px] shadow-xs transition-colors sm:text-sm",
        "placeholder:text-muted-foreground hover:border-ring/60 focus-visible:ring-ring/40 focus-visible:outline-none focus-visible:ring-2",
        "disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    />
  )
}

export { Input }
