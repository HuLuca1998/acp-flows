import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { freezeContract, getContract } from '@/api/system'
import type { Contract } from '@/models/contract'

import styles from './UnitContract.module.css'

export type UnitContractProps = {
  workID: string
  unitID: string
}

/**
 * 一个单元的契约：验收标准 + 写入边界。
 *
 * ★★ **边界要摆在明处**：它是用户唯一的防线——没有它，「AI 要动文件」
 * 只能靠他逐条判断。折叠起来的话，他不会去点开，
 * 而那等于把防线藏在了一次点击后面。
 *
 * ★ 冻结**由用户点**。冻结之后 AI 才能照着它开工，而冻结前它一个字都
 * 不该改——所以这个按钮是他把关的那一下。
 */
export function UnitContract({ workID, unitID }: UnitContractProps) {
  const { t } = useTranslation()
  const [contract, setContract] = useState<Contract | null>(null)
  const [open, setOpen] = useState(false)
  const [pending, setPending] = useState(false)
  const [errorCode, setErrorCode] = useState<string | null>(null)

  const load = useCallback(async () => {
    setErrorCode(null)
    try {
      setContract(await getContract(workID, unitID))
      setOpen(true)
    } catch (err) {
      // ★ 还没有契约是常态（单元设计师还没跑），说清楚而不是报错
      setErrorCode(codeOf(err))
      // ★★ **失败也要展开**：不展开的话用户点了「查看契约」之后
      // 界面毫无变化——他不知道是没点上、还是在转圈、还是没有契约。
      setOpen(true)
    }
  }, [workID, unitID])

  const freeze = useCallback(async () => {
    setPending(true)
    setErrorCode(null)
    try {
      setContract(await freezeContract(workID, unitID))
    } catch (err) {
      setErrorCode(codeOf(err))
    } finally {
      setPending(false)
    }
  }, [workID, unitID])

  if (!open) {
    return (
      <button type="button" className={styles.reveal} onClick={() => void load()}>
        {t('contract.view')}
      </button>
    )
  }
  if (contract === null) {
    return <span className={styles.error}>{t(problemKey(errorCode ?? ''))}</span>
  }

  return (
    <div className={styles.panel} data-frozen={contract.frozen}>
      <div className={styles.head}>
        <span className={styles.version}>{`contract v${contract.version}`}</span>
        {contract.frozen ? (
          <span className={styles.frozen}>{t('contract.frozen')}</span>
        ) : (
          <button type="button" className={styles.freeze} disabled={pending} onClick={() => void freeze()}>
            {t(pending ? 'contract.freezing' : 'contract.freeze')}
          </button>
        )}
      </div>

      <ul className={styles.criteria}>
        {contract.criteria.map((c) => (
          <li key={c.id} className={styles.criterion}>
            <span className={styles.critID}>{c.id}</span>
            <span>{c.text}</span>
          </li>
        ))}
      </ul>

      {/* ★★ 边界**不折叠**：它是用户唯一的防线，藏在一次点击后面等于没有 */}
      <div className={styles.boundary}>
        <span className={styles.boundaryTitle}>{t('contract.boundary')}</span>
        {contract.boundary.allowed.map((p) => (
          <span key={`a-${p}`} className={styles.allowed}>{p}</span>
        ))}
        {contract.boundary.forbidden.map((p) => (
          <span key={`f-${p}`} className={styles.forbidden}>{p}</span>
        ))}
        {/* ★ 一条允许项都没有时**明说**：空着的话用户以为「没限制」，
            而实际上是「一个字节都不许改」 */}
        {contract.boundary.allowed.length === 0 && (
          <span className={styles.nothingAllowed}>{t('contract.nothingAllowed')}</span>
        )}
      </div>

      {errorCode !== null && <span className={styles.error}>{t(problemKey(errorCode))}</span>}
    </div>
  )
}

/** 错误码 → 词条 key 的**显式映射**（不许模板拼接，check-i18n 会拦）。 */
const ERROR_KEY: Record<string, string> = {
  contract_not_found: 'contract.error.notFound',
  contract_empty: 'contract.error.empty',
  contract_already_frozen: 'contract.error.alreadyFrozen',
  contract_store_unavailable: 'contract.error.storeUnavailable',
}

function problemKey(code: string): string {
  return ERROR_KEY[code] ?? 'contract.error.failed'
}

function codeOf(err: unknown): string {
  if (err instanceof Error && err.message !== '') {
    return err.message
  }
  return 'contract_operation_failed'
}
