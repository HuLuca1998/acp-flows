//! duetd sidecar 的生命周期。
//!
//! 壳负责把后端拉起来、等它写出 `session.json`、把端口与 token 注入前端。
//!
//! ★ **注入的是 HTTP 连接信息，不是 IPC 通道。**
//! 前端拿到端口与 token 之后，访问后端的方式和浏览器完全一样（fetch + EventSource）。
//! 破坏这条，Web 版当天就废——而 Web 版是 AI 自测的主要通道。

use std::path::PathBuf;
use std::time::{Duration, Instant};

use serde::Deserialize;
use tauri::path::BaseDirectory;
use tauri::{Manager, Runtime};

/// duetd 写出的运行时会话信息。
#[derive(Debug, Deserialize)]
pub struct Session {
    pub port: u16,
    pub token: String,
    pub pid: u32,
}

/// 等 `session.json` 出现的上限。
///
/// duetd 要开库、跑迁移，冷启动可能上千毫秒。给足余量，
/// 但**必须有上限**——等不到就报错，不能让窗口空着转圈。
const SESSION_TIMEOUT: Duration = Duration::from_secs(20);

/// 轮询间隔。文件出现是瞬时事件，轮询比装 fs watcher 简单可靠。
const POLL_INTERVAL: Duration = Duration::from_millis(50);

/// sidecar 启动失败的原因。**每一条都要能让用户知道下一步做什么。**
#[derive(Debug)]
pub enum SidecarError {
    Spawn(String),
    Timeout,
    Malformed(String),
}

impl std::fmt::Display for SidecarError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Spawn(msg) => write!(f, "无法启动 duetd：{msg}"),
            Self::Timeout => write!(
                f,
                "duetd 启动超时（{}s）。请检查是否有旧进程占用数据目录",
                SESSION_TIMEOUT.as_secs()
            ),
            Self::Malformed(msg) => write!(f, "duetd 的 session.json 无法解析：{msg}"),
        }
    }
}

/// `~/.acpflows/runtime/session.json` 的路径。
///
/// 必须与后端 `platform.OSPaths.RuntimeSession()` 保持一致——
/// 两边各写各的路径，症状是「后端起来了但壳一直等」。
fn session_path<R: Runtime>(app: &tauri::AppHandle<R>) -> Result<PathBuf, SidecarError> {
    let home = app
        .path()
        .resolve("", BaseDirectory::Home)
        .map_err(|e| SidecarError::Spawn(format!("解析 home 目录失败：{e}")))?;
    Ok(home.join(".acpflows").join("runtime").join("session.json"))
}

/// 等 duetd 写出 session.json 并解析它。
///
/// 启动前会删掉旧文件：上一次运行残留的 session.json 会让壳
/// 连到一个已经死掉的端口，而症状是「界面一片空白但没有报错」。
fn await_session(path: &PathBuf) -> Result<Session, SidecarError> {
    let deadline = Instant::now() + SESSION_TIMEOUT;

    loop {
        if let Ok(raw) = std::fs::read_to_string(path) {
            // 文件可能被读到一半（duetd 正在写）。解析失败就继续等，
            // 不要立刻判失败——那会把一次时序竞争变成一次启动失败。
            if let Ok(session) = serde_json::from_str::<Session>(&raw) {
                return Ok(session);
            }
        }
        if Instant::now() >= deadline {
            // 文件在但一直解析不了 → 报「格式不对」而不是「超时」。
            // 两者的排查方向完全不同：前者去看 duetd 写了什么，
            // 后者去看它是不是根本没起来。
            return Err(match std::fs::read_to_string(path) {
                Ok(raw) if !raw.trim().is_empty() => SidecarError::Malformed(raw),
                _ => SidecarError::Timeout,
            });
        }
        std::thread::sleep(POLL_INTERVAL);
    }
}

/// 拉起 duetd 并返回它的连接信息。
///
/// `--updater` 告诉后端「本进程由 Tauri 壳拉起，具备自动更新能力」。
/// 不传的话检查更新会返回 `unsupported`，装出来的 app 永远更新不了。
pub fn spawn<R: Runtime>(app: &tauri::AppHandle<R>) -> Result<Session, SidecarError> {
    use tauri_plugin_shell::process::CommandEvent;
    use tauri_plugin_shell::ShellExt;

    let path = session_path(app)?;
    // 清掉上一次的残留，否则可能连到一个已经死掉的端口
    let _ = std::fs::remove_file(&path);

    let (mut rx, _child) = app
        .shell()
        .sidecar("duetd")
        .map_err(|e| SidecarError::Spawn(e.to_string()))?
        .args(["--updater"])
        .spawn()
        .map_err(|e| SidecarError::Spawn(e.to_string()))?;

    // ★ stderr 必须收走。duetd 的启动失败原因**只在 stderr 里**
    // （端口被占、数据库锁死、迁移失败）。不收的话用户只看到一个白窗口。
    tauri::async_runtime::spawn(async move {
        while let Some(event) = rx.recv().await {
            match event {
                CommandEvent::Stderr(line) => {
                    eprintln!("duetd: {}", String::from_utf8_lossy(&line));
                }
                CommandEvent::Terminated(payload) => {
                    eprintln!("duetd 退出，code={:?}", payload.code);
                    break;
                }
                _ => {}
            }
        }
    });

    await_session(&path)
}
