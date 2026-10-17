import type { LucideIcon } from "lucide-react"
import type { ReactNode } from "react"

/** 扩展卡片统一外壳：图标 + 标题 + 可选右上角链接，内容在下方。 */
export function BlockShell({
  icon: Icon,
  label,
  href,
  action,
  children,
}: {
  icon: LucideIcon
  label: string
  href?: string
  action?: string
  children: ReactNode
}) {
  return (
    <div className="border-border bg-card overflow-hidden rounded-xl border shadow-xs">
      <div className="bg-muted/60 text-muted-foreground flex items-center gap-1.5 border-b px-3 py-1.5 text-[11px] font-medium">
        <Icon className="size-3.5" />
        <span>{label}</span>
        {href && action && (
          <a
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            className="hover:text-foreground ml-auto text-[11px]"
          >
            {action}
          </a>
        )}
      </div>
      {children}
    </div>
  )
}
