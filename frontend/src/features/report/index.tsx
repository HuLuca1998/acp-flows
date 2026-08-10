import { Skeleton } from '@/ui/Skeleton'

/**
 * ReportPage。当前是骨架占位——**不含任何编造的数据**。
 *
 * 真正的实现见 旧计划文档（已随重设计移除） 的 M12 一节。
 */
export function ReportPage() {
  return <Skeleton hintKey="page.report.hint" />
}
