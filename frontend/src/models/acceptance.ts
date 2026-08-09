import type { components } from '@/api/gen/schema'

/** 一个单元的验收：标准与证据对照。 */
export type Acceptance = components['schemas']['Acceptance']

/** 一条证据。★ `trustworthy` 说它是不是应用直接采集的。 */
export type Evidence = components['schemas']['Evidence']
