import { Skeleton } from '@/ui/Skeleton'

/**
 * MemoryPage。当前是骨架占位——**不含任何编造的数据**。
 *
 * 真正的实现见 docs/plan/milestones/M5-polish.md。
 */
export function MemoryPage() {
  return <Skeleton hintKey="page.memory.hint" />
}
