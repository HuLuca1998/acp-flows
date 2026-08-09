import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { getPlan } from '@/api/system'
import type { Plan } from '@/models/plan'

import styles from './PlanPanel.module.css'

export type PlanPanelProps = {
  workID: string
}

/**
 * 计划面板：子计划 DAG 与它下面的单元。
 *
 * ★★ **每个单元都标着由哪个角色做**（裁定三）。不标的话，用户看不出
 * 「这条谁在干」——而那正是他判断「该不该信这个产出」的依据：
 * 实现方审查自己的产出是 INV-ATT-8 明令禁止的。
 *
 * ★ 进度与状态**后端给什么显示什么**，前端不自己算：两处各算一遍的话
 * 它们必然漂移，而漂移的那一刻用户看到的是「3/3 但还在跑」。
 */
export function PlanPanel({ workID }: PlanPanelProps) {
  const { t } = useTranslation()
  const [plan, setPlan] = useState<Plan | null>(null)

  const reload = useCallback(async () => {
    if (workID === '') {
      setPlan(null)
      return
    }
    try {
      setPlan(await getPlan(workID))
    } catch {
      // ★ 读不到就**不显示这一块**，不报错：还没规划是新工作的常态，
      // 而弹一句「读取计划失败」会让用户以为出了什么事。
      setPlan(null)
    }
  }, [workID])

  useEffect(() => {
    void reload()
  }, [reload])

  if (plan === null) {
    return null
  }

  return (
    <section className={styles.panel} aria-label={t('plan.title')}>
      <header className={styles.head}>
        {/* 等宽版本号照设计稿，不翻译 */}
        <span className={styles.version}>{`plan v${plan.version}`}</span>
        <span className={styles.counts}>
          {t('plan.counts', { subplans: plan.subplan_count, units: plan.unit_count })}
        </span>
        {plan.title !== '' && <span className={styles.title}>{plan.title}</span>}
      </header>

      {plan.subplans.map((sp) => (
        <div key={sp.id} className={styles.subplan} data-status={sp.status}>
          <div className={styles.subplanHead}>
            <span className={styles.id}>{sp.id}</span>
            <span className={styles.subplanTitle}>{sp.title}</span>
            {/* 设计稿的 `accepted · 3/3`。★ 状态词是**原值不翻译**（术语表） */}
            <span className={styles.status}>{sp.status}</span>
            <span className={styles.progress}>{`${sp.done}/${sp.total}`}</span>
          </div>

          <ul className={styles.units}>
            {sp.units.map((u) => (
              <li key={u.id} className={styles.unit} data-accepted={u.accepted}>
                <span className={styles.id}>{u.id}</span>
                <span className={styles.unitTitle}>{u.title}</span>
                {/*
                  ★★ 角色标签。认不出的角色**显示后端给的原始 id**，
                  不编一个名字——编出来的与角色页那张表对不上，
                  用户会以为有两个不同的角色。
                */}
                <span className={styles.role} data-role={u.role_id}>
                  {u.role_display_name === undefined || u.role_display_name === ''
                    ? u.role_id
                    : u.role_display_name}
                </span>
                {u.depends_on.length > 0 && (
                  <span className={styles.deps}>
                    {t('plan.dependsOn', { list: u.depends_on.join(' · ') })}
                  </span>
                )}
                {/* 设计稿的「契约未冻结」——冻没冻结决定这个单元能不能开工 */}
                {!u.contract_frozen && (
                  <span className={styles.unfrozen}>{t('plan.contractNotFrozen')}</span>
                )}
              </li>
            ))}
          </ul>
        </div>
      ))}
    </section>
  )
}
