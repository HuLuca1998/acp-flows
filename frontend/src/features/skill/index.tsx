import { Skeleton } from '@/ui/Skeleton'

/**
 * SkillPage。当前是骨架占位——**不含任何编造的数据**。
 *
 * 真正的实现见 docs/plan/milestones/M2-roles-skills-memory.md 的 U2.4.1。
 */
export function SkillPage() {
  return <Skeleton hintKey="page.skill.hint" />
}
