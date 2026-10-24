import { Download, PackageOpen } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { BackupSection } from "@/routes/backup-section"
import { MediaSection } from "@/routes/media"
import { ImportSection } from "@/routes/import"

/** 设置 · 数据：内容包导出、数据库备份、旧数据导入。 */
export function DataPage() {
  const { t } = useTranslation()

  return (
    <div className="space-y-4">
      <section className="wisp-card p-5">
        <h2 className="wisp-section-title flex items-center gap-1.5">
          <PackageOpen className="size-4" /> {t("data.capsuleTitle")}
        </h2>
        <p className="wisp-hint mt-1 mb-3">{t("data.capsuleHint")}</p>
        <Button asChild size="sm" variant="outline">
          <a href="/api/v1/admin/export/capsule" download>
            <Download /> {t("data.capsuleButton")}
          </a>
        </Button>
      </section>

      <BackupSection />

      <MediaSection />

      <section className="wisp-card p-5">
        <ImportSection />
      </section>
    </div>
  )
}
