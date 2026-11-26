# SC000 Proton 配置与启动记录

本机使用 Ubuntu 26.04.1、KDE Wayland、AMD Radeon RX 6800 和 Manjuu / SC000 正式版 `1.0.0.30` 进行测试。

| 项目 | 实测配置 |
|---|---|
| Proton | `dwproton-11.0-16`，Steam 的 `compatibilitytools.d` 安装目录 |
| 图形会话 | KDE Wayland，`PROTON_ENABLE_WAYLAND=1` |
| 游戏目标与工作目录 | 安装根目录下的 `AzurPromilia.exe`，工作目录为安装根目录 |
| 游戏启动参数 | `-start=azurpromilia_launcher -screen-fullscreen 0 -screen-width 1280 -screen-height 720` |
| Wine 依赖搜索路径 | 游戏自带的 `AzurPromilia_Data/Plugins/mj_sdk` |
| 独立前缀 | `$HOME/Games/compatdata/azurpromilia-sc000-dwproton` |
| 网络 | 启动进程清除 HTTP、HTTPS 和 ALL_PROXY 代理变量，本机实测直连 |

本机默认运行以下命令。将游戏目录换成实际安装位置。

```bash
./tools/run_proton.sh "$HOME/Games/AzurPromilia-test"
```

如需改 Steam 路径、Proton 版本或前缀，复制并编辑配置模板。

```bash
config_dir="${XDG_CONFIG_HOME:-$HOME/.config}/azurpromilia"
mkdir -p "$config_dir"
cp tools/proton.conf.example "$config_dir/proton.conf"
```

也可用 `AZUR_PROTON_CONFIG=/path/to/proton.conf` 指定配置文件。配置以 Bash 语法读取，使用自己保存的文件。重新启动时沿用同一个前缀。保留运行原生 Wayland 所需的桌面会话环境。

## 2026-11-26 实测

- Valve Proton 11.0-3 在 ACE 初始化阶段退出。按本机原有《鸣潮》启动方式使用 `dwproton-11.0-16` 和新建的《蓝色星原》前缀，Unity 能启动。XWayland 配置在首次协议界面黑屏，改为 Wine 原生 Wayland 后可见游戏加载画面。
- 客户端进入 SDK 登录流程。首次进入后，游戏内资源更新要求约 16.13 GB 可用空间。空间足够后，客户端下载、导入资源并清理了更新缓存。
- 更新后使用同一前缀重启，客户端复用了已导入的资源，完成着色器预热。启动日志进入 `FsmUpdaterDone`。
