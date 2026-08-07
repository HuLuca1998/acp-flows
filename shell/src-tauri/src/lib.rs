//! Duet 的 Tauri 外壳。
//!
//! **壳只做四件事**：窗口、原生能力、sidecar 生命周期、自动更新。
//! 不写业务逻辑、不做持久化、不直接和 ACP Runtime 说话。
//!
//! ★ 最重要的约束：**不得通过 Rust IPC 绕过 HTTP 调用后端**。
//! 前端访问后端的方式必须和浏览器完全一样（fetch + EventSource 打 127.0.0.1）。
//! 破坏这条，Web 版当天就废——而 Web 版是 AI 自测的主要通道。
//! 见 docs/spec/architecture.md §1 与 shell/AGENTS.md。

mod sidecar;
mod tray;

use tauri::{Emitter, Manager, WindowEvent};

/// 启动应用。
///
/// TODO(M1 U1.1.2): 自绘 42px 窗口栏的拖拽区。
#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_updater::Builder::new().build())
        .setup(|app| {
            let handle = app.handle().clone();

            match sidecar::spawn(&handle) {
                Ok(session) => {
                    // 把连接信息注入前端。**注入的是 HTTP 端口与 token，不是 IPC 通道**——
                    // 前端拿它去 fetch，和在浏览器里跑没有区别。
                    let script = format!(
                        "window.__DUET__ = {{ port: {}, token: {:?}, pid: {} }};",
                        session.port, session.token, session.pid
                    );
                    if let Some(window) = handle.get_webview_window("main") {
                        // 注入失败不致命：前端会退回开发态默认值并显示连不上，
                        // 那比一个没有任何提示的白窗口强。
                        if let Err(e) = window.eval(&script) {
                            eprintln!("duet: 注入连接信息失败: {e}");
                        }
                    }
                }
                Err(e) => {
                    // ★ 起不来时**如实告诉用户**，不要留一个空窗口让他猜。
                    eprintln!("duet: {e}");
                    let _ = handle.emit("duet://sidecar-failed", e.to_string());
                }
            }

            // 菜单栏常驻。失败不致命——没有托盘图标应用照样能用，
            // 只是关窗口后没法再唤出来，所以要如实报出去。
            if let Err(e) = tray::setup(&handle) {
                eprintln!("duet: 菜单栏图标创建失败: {e}");
            }

            Ok(())
        })
        .on_window_event(|window, event| {
            // ★ **关窗口 ≠ 退出应用。**
            // 后台可能有正在跑的 Agent 与未落盘的工作，点一下红叉就全杀掉，
            // 用户会毫无预警地丢掉几十分钟的活。这里拦下关闭、把应用收进菜单栏，
            // 它继续在后台跑。真正退出走托盘菜单的「强制退出」。
            if let WindowEvent::CloseRequested { api, .. } = event {
                api.prevent_close();
                // 收进菜单栏 = 隐藏窗口 **且** 让 Dock 图标消失。
                // 只 hide 窗口的话 Dock 里还占着一格，看起来像没关干净。
                tray::hide_to_tray(window.app_handle());
            }
        })
        .run(tauri::generate_context!())
        .unwrap_or_else(|e| {
            // 壳起不来时给出可诊断的信息，而不是静默退出。
            eprintln!("duet: 启动失败: {e}");
            std::process::exit(1);
        });
}
