# SC000 手动 Proton 配置参考

> 下载器只负责下载、安装、校验、修复和版本更新。它不会启动游戏、安装 Proton、修改 Steam 配置或创建 CompatData 前缀。

## 已验证的配置

测试环境：Ubuntu 26.04.1、KDE Wayland、AMD Radeon RX 6800、Manjuu / SC000 正式版 `1.0.0.30`。

| 项目 | 实测配置 |
|---|---|
| 游戏入口 | 安装根目录的 `AzurPromilia.exe` |
| 工作目录 | 游戏安装根目录 |
| Proton | `dwproton-11.0-16` |
| 图形会话 | KDE Wayland，启用 `PROTON_ENABLE_WAYLAND=1` |
| 独立前缀 | `$HOME/Games/compatdata/azurpromilia-sc000-dwproton` |
| SDK 搜索路径 | `AzurPromilia_Data/Plugins/mj_sdk` |
| 游戏参数 | `-start=azurpromilia_launcher -screen-fullscreen 0 -screen-width 1280 -screen-height 720` |

这是已验证机器上的配置参考，不代表所有显卡、桌面环境和 Proton 版本都已验证。

## Steam 手动配置

1. 确认下载器的 `install` 和 `verify` 均已成功。
2. 在 Steam 中选择“添加非 Steam 游戏”，浏览到游戏目录下的 `AzurPromilia.exe`。
3. 打开该快捷方式的属性，将“起始位置/工作目录”设为游戏安装根目录。
4. 在“兼容性”中勾选强制使用指定兼容工具，选择已安装的 `dwproton-11.0-16`。
5. 将 [`tools/steam-launch-options.example`](../tools/steam-launch-options.example) 中的启动参数复制到“启动选项”。
6. 把示例中的 `YOUR_USER` 和游戏路径替换成自己机器上的真实路径。不要照抄其他机器的绝对路径。
7. 在 Wayland 会话下保留 `PROTON_ENABLE_WAYLAND=1`；如果使用 X11，可先移除该变量再单独测试。

启动选项中有三类配置：

- `STEAM_COMPAT_DATA_PATH`：使用独立 Wine/Proton 前缀，避免和其他游戏共用配置。
- `WINEPATH`：指向游戏自带的 `AzurPromilia_Data/Plugins/mj_sdk`，供 SDK 相关依赖查找。
- `%command%` 后的参数：指定游戏启动入口和窗口模式。

如需诊断，可在启动选项最前面临时添加 `PROTON_LOG=1`。日志可能包含本机路径等信息，公开前请检查并脱敏。

## 已知实测现象

- Valve Proton `11.0-3` 在 ACE 初始化阶段退出；本机改用 `dwproton-11.0-16` 后能够进入客户端 SDK 登录流程。
- XWayland 配置在首次协议界面出现黑屏；启用原生 Wayland 后可见游戏加载画面。
- 客户端首次进入后，游戏内资源更新还需要约 **16.13 GB** 可用空间。空间足够后，客户端完成下载、资源导入并清理更新缓存。
- 更新后沿用同一个 CompatData 前缀重启，客户端复用了已导入资源并完成着色器预热，日志进入 `FsmUpdaterDone`。
- 用户随后反馈已成功进入并运行游戏。

游戏内资源更新属于客户端自身流程，不由 Linux 下载器接管。请为它额外预留空间；下载器显示的资源包大小不包含这部分客户端更新缓存。

## 不在下载器职责内

- 自动安装或更新 Steam / Proton；
- 自动创建、迁移或清理 CompatData；
- 自动添加 Steam 非 Steam 游戏快捷方式；
- 自动修改桌面环境、Wayland/XWayland 或代理设置；
- 自动启动游戏或处理账号登录。

本项目只维护本文件和启动选项示例，具体设置由用户按自己的 Steam、Proton 安装路径和桌面环境手动完成。
