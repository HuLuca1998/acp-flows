//! 菜单栏常驻与退出策略。
//!
//! **关窗口 ≠ 退出应用。** Duet 后台有正在跑的 Agent 与未落盘的工作，
//! 点一下红叉就把它们全杀掉，用户会在毫无预警的情况下丢掉几十分钟的活。
//! 所以关窗口只是收起来，应用继续在菜单栏里跑；真正要退出走菜单里的「强制退出」。

use tauri::menu::{Menu, MenuItem, PredefinedMenuItem};
use tauri::tray::{TrayIcon, TrayIconEvent};
use tauri::{AppHandle, Manager, Runtime};

/// 菜单项标识。用常量而不是散落的字面量——拼错一个字符就是一个点不动的菜单项。
const ID_SHOW: &str = "show";
const ID_QUIT: &str = "quit";

/// 把主窗口显示出来并聚焦。
///
/// 三步缺一不可：`show` 让它从隐藏态回来，`unminimize` 处理最小化，
/// `set_focus` 把它提到最前——只调 `show` 的话，窗口会出现在其他应用后面。
pub fn show_main_window<R: Runtime>(app: &AppHandle<R>) {
    // ★ 先回到 Regular：Accessory 策略下窗口**无法获得焦点**，
    // 只 show 不切策略的话，窗口会出现但点不动、也抢不到键盘。
    #[cfg(target_os = "macos")]
    let _ = app.set_activation_policy(tauri::ActivationPolicy::Regular);

    if let Some(window) = app.get_webview_window("main") {
        let _ = window.show();
        let _ = window.unminimize();
        let _ = window.set_focus();
    }
}

/// 把应用收进菜单栏：隐藏窗口，并让 Dock 图标消失。
///
/// ★ 只 `hide()` 窗口是不够的——Dock 图标会**留在原地**，
/// 用户看到的是「关了但还占着 Dock 一格」。
/// macOS 上「只活在菜单栏」的应用用的是 Accessory 激活策略。
pub fn hide_to_tray<R: Runtime>(app: &AppHandle<R>) {
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.hide();
    }
    #[cfg(target_os = "macos")]
    let _ = app.set_activation_policy(tauri::ActivationPolicy::Accessory);
}

/// 构建菜单栏图标与菜单。
pub fn setup<R: Runtime>(app: &AppHandle<R>) -> tauri::Result<TrayIcon<R>> {
    let show = MenuItem::with_id(app, ID_SHOW, "打开 Duet", true, None::<&str>)?;
    let separator = PredefinedMenuItem::separator(app)?;
    // ★ 文案写清后果。设计规范 §09：破坏性动作要在按钮上说明会发生什么，
    // 不用「退出」这种看不出代价的词。
    let quit = MenuItem::with_id(app, ID_QUIT, "强制退出（结束后台任务）", true, None::<&str>)?;
    let menu = Menu::with_items(app, &[&show, &separator, &quit])?;

    let tray = tauri::tray::TrayIconBuilder::with_id("duet-tray")
        // ★ 用**专门的 template 图**，不是 default_window_icon()。
        // 窗口图标是彩色带底板的（Dock 里用），塞进菜单栏会是一个突兀的方块。
        .icon(tauri::image::Image::from_bytes(include_bytes!(
            "../icons/tray@2x.png"
        ))?)
        // macOS 的菜单栏图标必须是 template：系统按明暗主题自动反色。
        // 不设的话深色菜单栏上会是一团糊。
        .icon_as_template(true)
        .menu(&menu)
        // 左键直接唤出窗口，右键才出菜单——这是 macOS 上的常见预期。
        .show_menu_on_left_click(false)
        .on_menu_event(|app, event| match event.id().as_ref() {
            ID_SHOW => show_main_window(app),
            ID_QUIT => {
                // 这里是**唯一**真正退出的入口。
                // 退出前不做「静默保存」之类的补救——那属于业务逻辑，
                // 壳不碰（shell/AGENTS.md）。菜单文案已经说清了后果。
                app.exit(0);
            }
            _ => {}
        })
        .on_tray_icon_event(|tray, event| {
            // 左键点图标：唤出窗口。
            if let TrayIconEvent::Click {
                button: tauri::tray::MouseButton::Left,
                button_state: tauri::tray::MouseButtonState::Up,
                ..
            } = event
            {
                show_main_window(tray.app_handle());
            }
        })
        .build(app)?;

    Ok(tray)
}
