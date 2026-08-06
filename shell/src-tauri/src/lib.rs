//! Duet 的 Tauri 外壳。
//!
//! **壳只做四件事**：窗口、原生能力、sidecar 生命周期、自动更新。
//! 不写业务逻辑、不做持久化、不直接和 ACP Runtime 说话。
//!
//! ★ 最重要的约束：**不得通过 Rust IPC 绕过 HTTP 调用后端**。
//! 前端访问后端的方式必须和浏览器完全一样（fetch + EventSource 打 127.0.0.1）。
//! 破坏这条，Web 版当天就废——而 Web 版是 AI 自测的主要通道。
//! 见 docs/architecture.md §1 与 shell/AGENTS.md。

/// 启动应用。
///
/// TODO(M1 U1.3.2): 拉起 duetd sidecar、读 session.json、注入 window.__DUET__。
/// TODO(M1 U1.3.1): 自绘 42px 窗口栏的拖拽区。
/// TODO(M1 U1.8.x): updater 流程——**更新前必须先调后端 prepare**，不是先下载。
#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_updater::Builder::new().build())
        .run(tauri::generate_context!())
        .unwrap_or_else(|e| {
            // 壳起不来时给出可诊断的信息，而不是静默退出。
            eprintln!("duet: 启动失败: {e}");
            std::process::exit(1);
        });
}
