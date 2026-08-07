import { Skeleton } from '@/ui/Skeleton'

/**
 * ChatPage。当前是骨架占位——**不含任何编造的数据**。
 *
 * 真正的实现见 docs/plan/milestones/M2-talk-and-observe.md。
 */
export function ChatPage() {
  return <Skeleton hintKey="page.chat.hint" />
}
