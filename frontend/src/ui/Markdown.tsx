import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import styles from './Markdown.module.css'

/**
 * Markdown 显示。设计规范 §05 / 实现规格 §7.8。
 *
 * - **默认渲染显示**，「查看源码」切到原文——这是必须有的，不是可选（§7.8）
 * - 渲染规格：标题 15/500、正文 13/1.7、行内 code 为 raised 底芯片、
 *   引用为左侧 2px 竖条
 * - ★ **不引第三方 markdown 库**：换来的是一个完整 HTML 管线去渲染
 *   用户/AI 提供的文本。这里只认标题、列表、围栏代码、引用、行内 code——
 *   SKILL.md 与记忆正文用不到更多，认不出的行按普通段落显示，不丢内容。
 */
export type MarkdownProps = {
  source: string
  mode: 'rendered' | 'source'
  onToggleMode: () => void
}

export function Markdown({ source, mode, onToggleMode }: MarkdownProps) {
  const { t } = useTranslation()
  return (
    <div className={styles.wrap}>
      <div className={styles.bar}>
        <button type="button" className={styles.toggle} onClick={onToggleMode}>
          {mode === 'rendered' ? t('ui.markdown.viewSource') : t('ui.markdown.viewRendered')}
        </button>
      </div>
      {mode === 'source' ? (
        <pre className={styles.source}>{source}</pre>
      ) : (
        <div className={styles.rendered}>{renderBlocks(source)}</div>
      )}
    </div>
  )
}

/** 行内片段：反引号切开，奇数段是 code 芯片。 */
function inline(text: string, key: number): ReactNode {
  const parts = text.split('`')
  if (parts.length === 1) {
    return text
  }
  return (
    <span key={key}>
      {parts.map((p, i) => (i % 2 === 1 ? <code key={i}>{p}</code> : p))}
    </span>
  )
}

type Block =
  | { kind: 'heading'; depth: number; text: string }
  | { kind: 'code'; lines: string[] }
  | { kind: 'quote'; lines: string[] }
  | { kind: 'list'; ordered: boolean; items: string[] }
  | { kind: 'para'; lines: string[] }

/** 把源文切成块。认不出的行归入段落，**不丢内容**。 */
function parseBlocks(source: string): Block[] {
  const blocks: Block[] = []
  const lines = source.split('\n')
  const at = (i: number): string => lines[i] ?? ''
  let i = 0
  while (i < lines.length) {
    const line = at(i)
    if (line.trim() === '') {
      i += 1
      continue
    }
    if (line.startsWith('```')) {
      const code: string[] = []
      i += 1
      while (i < lines.length && !at(i).startsWith('```')) {
        code.push(at(i))
        i += 1
      }
      i += 1 // 吃掉收尾围栏
      blocks.push({ kind: 'code', lines: code })
      continue
    }
    const heading = /^(#{1,4})\s+(.*)$/.exec(line)
    if (heading) {
      blocks.push({ kind: 'heading', depth: (heading[1] ?? '#').length, text: heading[2] ?? '' })
      i += 1
      continue
    }
    if (/^>\s?/.test(line)) {
      const quote: string[] = []
      while (i < lines.length && /^>\s?/.test(at(i))) {
        quote.push(at(i).replace(/^>\s?/, ''))
        i += 1
      }
      blocks.push({ kind: 'quote', lines: quote })
      continue
    }
    const ordered = /^\d+[.)]\s+/.test(line)
    if (ordered || /^[-*]\s+/.test(line)) {
      const items: string[] = []
      const re = ordered ? /^\d+[.)]\s+/ : /^[-*]\s+/
      while (i < lines.length && re.test(at(i))) {
        items.push(at(i).replace(re, ''))
        i += 1
      }
      blocks.push({ kind: 'list', ordered, items })
      continue
    }
    const para: string[] = []
    while (i < lines.length && at(i).trim() !== '') {
      para.push(at(i))
      i += 1
    }
    blocks.push({ kind: 'para', lines: para })
  }
  return blocks
}

function renderBlocks(source: string): ReactNode[] {
  return parseBlocks(source).map((b, key) => {
    switch (b.kind) {
      case 'heading': {
        // # → h3 起步：详情栏的页面标题在它上面，层级不越位。
        const levels = ['h3', 'h4', 'h5', 'h6'] as const
        const H = levels[Math.min(b.depth - 1, levels.length - 1)] ?? 'h6'
        return <H key={key}>{inline(b.text, key)}</H>
      }
      case 'code':
        return <pre key={key}>{b.lines.join('\n')}</pre>
      case 'quote':
        return <blockquote key={key}>{b.lines.map((l, i) => inline(l, i))}</blockquote>
      case 'list': {
        const items = b.items.map((it, i) => <li key={i}>{inline(it, i)}</li>)
        return b.ordered ? <ol key={key}>{items}</ol> : <ul key={key}>{items}</ul>
      }
      case 'para':
        return <p key={key}>{inline(b.lines.join(' '), key)}</p>
    }
  })
}
