# 玩家使用指南（Manjuu / SC000）

> 非官方 Linux 下载器。当前只针对 Manjuu 官方 PC 启动器的 SC000 渠道；不支持 Yostar 日本服。请先阅读限制，不要把尚未实测的功能用于唯一的正式游戏目录。

## 1. 当前支持状态

| 项目 | 状态 |
|---|---|
| Linux x86-64 CLI | 已提供 |
| SC000 官方匿名包信息获取 | 已实测 |
| REALITY 逐文件完整安装 | 已实测通过 |
| 完整文件 MD5 校验 | 40,457 / 40,457 通过 |
| 单文件 Repair | 已实测通过 |
| Steam / Proton 手动启动配置 | 已在一台 Linux 机器上进入并运行游戏 |
| SLICE 全量安装 | 尚未完成实服验收 |
| Preload / 跨版本 Update | 尚未完成实服验收 |
| Yostar 日本服 | 不支持，暂缓研究 |

真实安装样本为 Manjuu / SC000 1.0.0.30。当时服务器返回 downloadMode=2，即 REALITY 逐文件模式。程序默认使用服务器返回的模式自动选择；若服务器以后切换到 SLICE，虽然代码已有实现，但完整 SLICE 安装尚未通过实服验收。

## 2. 需要准备什么

- Linux x86-64。当前实测环境为 Ubuntu 26.04.1；其他发行版未逐一验证。
- Python 3。
- 64 位 Wine。Wine 10.0 + 当前 SC000 样本已验证。
- Manjuu 官方 SC000 Windows 安装器 EXE。请从官方渠道自行获取；本项目不分发游戏文件或原版安装器。
- 足够的磁盘空间和稳定网络。
- Steam 与 Proton：只在你希望通过 Steam/Proton 启动游戏时需要。

**不需要**安装 Go、Node.js 或 Electron，前提是使用 Release 中的预编译 Linux amd64 程序。Go 只在从源码构建时需要；Node.js 只用于开发检查，不是玩家运行时依赖。

已验证的安装器样本：

- 文件名：AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe
- SHA-256：cf4b1f6233af2f2cb2d1c0de12f54d07606c3c84c829390b6caccfb2d0d1f04b

更新的官方安装器可能改变内部封装；如果引导失败，请提交安装器版本和 SHA-256，不要改用 Yostar 安装器。

## 3. 下载并解压项目

从 GitHub Releases 下载名称包含 linux-amd64 的压缩包和同一 Release 的 SHA256SUMS。

解压后请保留目录结构，尤其不要把 azurpromilia 单独移动到其他地方：

~~~text
azurpromilia-<version>-linux-amd64/
├── azurpromilia
├── README.md
├── SHA256SUMS
├── docs/
│   ├── PLAYER_GUIDE.md
│   └── PROTON_SETUP.md
└── tools/
    ├── bootstrap.py
    ├── fetch_packageconfig.cjs
    └── steam-launch-options.example
~~~

在压缩包所在目录校验下载：

~~~bash
sha256sum -c SHA256SUMS
~~~

进入解压目录：

~~~bash
cd azurpromilia-<version>-linux-amd64
chmod +x ./azurpromilia
./azurpromilia help
~~~

如果尚无公开 Release，可临时从 [当前成功构建（Actions run ID：45788177480）](https://github.com/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux/actions/runs/45788177480) 下载 artifact azurpromilia-linux（有效期至 2027-02-24），也可以从 [Actions 构建记录](https://github.com/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux/actions/workflows/build.yml) 选择更新的成功构建。解压 artifact 后，先运行 `sha256sum -c PACKAGE_SHA256SUMS`，再解压其中的 `azurpromilia-linux-amd64.tar.gz`。Actions artifact 有保留期限。也可以按本文最后的“从源码构建”自行编译。

## 4. 获取官方 SC000 安装器

将你从 Manjuu 官方渠道取得的 Windows SC000 安装器放在本机，例如：

~~~text
$HOME/Downloads/AzurPromilia_manjuu_installer.exe
~~~

下载器会从这个 EXE 中读取原版启动器携带的参数，并通过 Wine 运行其 Electron/N-API 组件，向官方接口发起匿名请求，取得当前包清单。

这一步不会安装 Windows 启动器，也不会下载游戏资源。游戏账号登录仍由游戏客户端处理。

## 5. 查看服务器信息

在解压后的项目目录中运行：

~~~bash
GAME_DIR="$HOME/Games/AzurPromilia"
INSTALLER="$HOME/Downloads/AzurPromilia_manjuu_installer.exe"

./azurpromilia info \
  --installer "$INSTALLER" \
  --root "$GAME_DIR"
~~~

首次运行会调用 tools/bootstrap.py，需要 Python 3 和 Wine。官方元数据会缓存到游戏目录的 .azurpromilia/packageconfig.json，每次工作流默认刷新。

检查输出中的：

- Released 版本；
- Download mode；
- Package type；
- 游戏目录名与游戏入口。

当前已实测的 SC000 版本使用 REALITY / packageType 2。

## 6. 首次下载和安装

先确认目标目录是你准备给本项目使用的目录。不要直接指向一个已有的 Windows 或其他启动器管理的游戏目录。

~~~bash
GAME_DIR="$HOME/Games/AzurPromilia"
INSTALLER="$HOME/Downloads/AzurPromilia_manjuu_installer.exe"

./azurpromilia install \
  --installer "$INSTALLER" \
  --root "$GAME_DIR" \
  --workers 4
~~~

默认 --mode auto，会按官方元数据中的 downloadMode 选择包类型。只有确认自己要覆盖服务器默认模式时才使用 --mode reality 或 --mode slice。

下载完成后，程序会保存版本索引和本地 manifest。不要手工删除游戏目录中的 .azurpromilia、_index 等状态目录。

### 磁盘空间

已实测的 SC000 REALITY 安装传输了约 **84.84 GB** 资源。首次启动游戏客户端后，还需要额外下载约 **16.13 GB** 的客户端资源更新。请在游戏资源之外再预留空间，并为文件系统留出余量；实际需求会随官方版本变化。

SLICE 模式的当前样本包含 752 个分片，合并 ZIP 为 75,145,275,761 bytes。空目录安装时，分片下载与 ZIP 合并阶段的峰值至少约 **150.29 GB（140.0 GiB）**，还不包含解压空间。SLICE 全量安装尚未实测通过，不建议手动强制选择它。

## 7. 校验和修复

安装成功后执行完整校验：

~~~bash
./azurpromilia verify --root "$HOME/Games/AzurPromilia"
~~~

当前版本的 REALITY 安装已实测 40,457 个文件全部通过大小和 MD5 校验。

如果同一个服务器版本的游戏文件缺失或损坏，可运行：

~~~bash
./azurpromilia repair \
  --installer "$HOME/Downloads/AzurPromilia_manjuu_installer.exe" \
  --root "$HOME/Games/AzurPromilia" \
  --workers 4
~~~

Repair 会重新获取当前官方索引，只补齐缺失或损坏的资源。如果服务器已经发布新版本，Repair 会拒绝跨版本修复并提示使用 Update。

## 8. Steam / Proton 手动配置

下载器不会安装 Proton、创建 CompatData、添加 Steam 快捷方式或启动游戏。完成安装和 Verify 后，请按自己的 Steam 环境手动配置：

1. 在 Steam 中选择“添加非 Steam 游戏”，选择游戏目录下的 AzurPromilia.exe。
2. 将“起始位置/工作目录”设为游戏安装根目录。
3. 在“兼容性”中选择已安装的 Proton 兼容工具。
4. 将 tools/steam-launch-options.example 的内容复制到启动选项。
5. 将示例中的 YOUR_USER、游戏目录和 CompatData 路径替换成自己机器上的实际路径。

已验证的一套配置使用 dwproton-11.0-16、KDE Wayland、独立 CompatData 和 PROTON_ENABLE_WAYLAND=1。这只是一个实测配置，不保证适用于所有发行版、显卡和 Proton 版本。

完整说明见 [Proton 手动配置参考](PROTON_SETUP.md)。

## 9. 暂不建议用于正式目录的功能

代码中已经实现 preload 和 update，但尚未完成真实版本切换的完整验收。它们涉及新旧版本索引、预下载缓存复用和旧文件清理。

在专门的真实版本升级测试完成前：

- 不要把 preload / update 当作已稳定功能；
- 不要在唯一的正式游戏目录上试验；
- 先备份游戏目录和存档（如果存档位于游戏目录之外，也请确认其位置）；
- 不要手工将 _pre_download 内容覆盖到游戏目录。

## 10. 隐私与安全说明

- packageconfig.json 可能包含临时 CDN URL。不要公开上传或提交到 Git。
- 不要分享原版安装器中的 app_config.dat、密钥、账号 Cookie、token 或签名请求头。
- 本工具使用官方索引中的 MD5 和 ZIP CRC32 做资源完整性检查；它们不是数字签名。
- 本工具不处理账号登录，也不修改游戏反作弊或游戏客户端代码。
- 只使用官方来源的安装器和游戏资源。

## 11. 从源码构建

如不使用 Release 预编译包，需要 Go 1.22 或更新版本：

~~~bash
git clone https://github.com/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux.git
cd AzurPromilia_manjuu_installer_linux

go test -race ./...
go vet ./...
CGO_ENABLED=0 go build -trimpath -o ./azurpromilia ./cmd/azurpromilia

./azurpromilia help
~~~

运行时仍需 Python 3、Wine 和官方 SC000 安装器。请保留仓库中的 tools/ 目录与二进制相对位置。

## 12. 问题反馈

提交 Issue 时请附上：

- Linux 发行版和 CPU 架构；
- Wine 版本；
- 使用的官方 SC000 安装器版本及 SHA-256；
- 完整命令（可删除用户名和私人路径）；
- 完整错误日志与失败阶段。

不要直接上传原始 packageconfig.json、安装器内部配置、账号凭据或包含临时签名参数的请求记录。
