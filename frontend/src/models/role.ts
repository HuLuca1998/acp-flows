import type { components } from '@/api/gen/schema'

/** 一个角色。形状由 api/openapi.yaml 决定。 */
export type Role = components['schemas']['Role']

/**
 * 语义档位。★ **不是某一端的档名**——两端档名一个都不重合。
 *
 * 要显示实际档名时用 `role.mode_name`，那是后端翻译好的。
 * 前端自己翻译就得认识 `plan` / `read-only` 这些品牌相关的取值，
 * 而那正是分层要挡住的东西。
 */
export type SessionMode = NonNullable<Role['session_mode']>

/**
 * 这个角色能写文件吗。
 *
 * ★ 判断依据是**语义档**，不是 `mode_name`——按档名判断的话，
 * 加一端就要加一串 `||`，而漏一个的表现是「界面说它只读，实际它能写」。
 */
export function canWrite(role: Role): boolean {
  return role.session_mode !== 'read_only'
}
