/** 权限选项的类别，与 ACP 的 `PermissionOptionKind` 一致。 */
export type PermissionOptionKind =
  | 'allow_once'
  | 'allow_always'
  | 'reject_once'
  | 'reject_always'

/**
 * 一个可选应答。
 *
 * ★ `optionId` 由 Agent 定义，是**不透明字符串**——原样回传，不要自己造，
 * 也不要按 `kind` 重新匹配。搞错的话，用户点「拒绝」而 Agent 收到「允许」。
 */
export type PermissionOption = {
  optionId: string
  /** Agent 给的按钮文字。**照它显示**，不要替换成我们自己的说法。 */
  name: string
  kind: PermissionOptionKind
}

/** 工具类别，与 ACP 的 `ToolKind` 一致。 */
export type PermissionToolKind =
  | 'read'
  | 'edit'
  | 'delete'
  | 'move'
  | 'search'
  | 'execute'
  | 'think'
  | 'fetch'
  | 'switch_mode'
  | 'other'

/** 一次待裁决的权限请求。 */
export type PermissionRequest = {
  /** 这次请求的唯一标识，应答时带回去。 */
  id: string
  toolCallId: string
  /** 哪个 Agent 在问——用户可能同时开着几个。 */
  runtime: string
  kind: PermissionToolKind
  /** 涉及的文件路径；Agent 不一定给得出（比如执行命令）。 */
  path?: string
  /**
   * 是否越出了这个单元声明的写入边界。
   *
   * ★ 没有依据时**不要填 true**：「写入边界外」是一句很重的话，
   * 乱说的话用户会对所有提示脱敏，真正越界那次他也不会看。
   */
  outOfBounds?: boolean
  options: PermissionOption[]
}
