# 玩家使用指南（Manjuu / SC000）

> 非官方 Linux 下载器。当前只针对 Manjuu 官方 PC 启动器的 SC000 渠道；不支持 Yostar 日本服。请先阅读限制，不要把尚未实测的功能用于唯一的正式游戏目录。

## 1. 当前支持状态

| 项目 | 状态 |
|---|---|
| Linux x86-64 CLI | 已提供 |
| Linux AArch64 CLI | 已提供。官方 x86-64 启动器自动引导需另有兼容运行环境 |
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

- Linux x86-64 或 AArch64。官方自动引导在 Ubuntu 26.04.1 x86-64 上实测。
- Python 3。
- 能运行官方 x86-64 Windows 启动器的 Wine 环境。x86-64 上已通过 Wine 10.0 实测。
- 足够的磁盘空间和稳定网络。
- Steam 与 Proton：只在你希望通过 Steam/Proton 启动游戏时需要。

预编译包可直接运行下载器。首次自动引导下载 Windows 启动器，并通过 Wine 执行它内置的 Electron 运行时。Go 用于从源码构建。

arm64 上若暂时没有适合官方启动器的运行环境，可先在 amd64 机器取得官方 `packageconfig.json` 和对应的启动器，再为下载命令同时指定 `--packageconfig` 与 `--installer`。

## 3. 下载并解压项目

下载与你的系统相符的 `linux-amd64` 或 `linux-arm64` 压缩包，以及该版本的 `SHA256SUMS`。[GitHub Releases](https://github.com/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux/releases)

解压后请保留目录结构，尤其不要把可执行文件单独移动到其他地方。Release 包同时提供推荐短命令 `azp` 和兼容用完整命令 `azurpromilia`：

~~~text
azurpromilia-<version>-linux-<arch>/
├── azp -> azurpromilia
├── azurpromilia
├── README.md
├── LICENSE
├── SHA256SUMS
├── docs/
│   ├── PLAYER_GUIDE.md
│   └── PROTON_SETUP.md
└── tools/
    ├── bootstrap.py
    ├── fetch_packageconfig.cjs
    └── steam-launch-options.example
~~~

在下载目录校验并解压。把示例中的版本和架构改成你下载的包。

~~~bash
package=azurpromilia-v0.1.0-preview.1-linux-amd64
sha256sum --ignore-missing -c SHA256SUMS
tar -xzf "$package.tar.gz"
cd "$package"
sha256sum -c SHA256SUMS
./azp --help
~~~

要测试尚未发布的 main 分支，可下载最新成功构建的 `azurpromilia-linux` artifact。解压 artifact 后，先运行 `sha256sum -c PACKAGE_SHA256SUMS`，再解压其中的 `azurpromilia-linux-amd64.tar.gz`。[Actions 构建记录](https://github.com/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux/actions/workflows/build.yml)

## 4. 交互菜单

在真实终端里直接运行 `./azp` 会进入交互菜单。使用 ↑/↓ 移动、Enter 确认、Esc 或 q 退出。

菜单会根据本地状态调整可用项：

- 未设置游戏目录时，优先引导设置目录；
- 尚未安装时允许安装，但更新、校验和修复会提示先安装；
- 已安装后默认高亮 Status，并开放 Update / Verify / Sync；
- 脚本、管道或 CI 等非交互环境裸跑时只输出帮助，不进入菜单。

## 5. 官方 SC000 启动器自动获取

普通玩家**不需要自己寻找或下载 Windows 启动器**。

第一次执行需要官方元数据的命令时，程序会访问 Manjuu 官网使用的生产下载接口，获取当前 `SC000` PC 启动器 URL，将它下载到用户缓存目录并验证结构。后续运行会复用已经验证的缓存。

如果官网发现接口暂时不可用，程序会使用项目内置的最后一个已验证官方来源作为兜底。目前维护的兜底版本是：

- SC000 启动器版本：`1.0.2.2`
- 大小：`190835304` bytes
- SHA-256：`0a1d1b97947838146c7b8f3f0d27dbba79e60372d043f60cd5401f2979531282`

常见 Linux 环境中缓存位于 `$XDG_CACHE_HOME/azurpromilia/SC000`；未设置 `XDG_CACHE_HOME` 时通常是 `~/.cache/azurpromilia/SC000`。第一次自动下载约需额外 182 MiB 网络流量。

自动下载的 EXE **不会被安装**。程序只读取其中的启动器参数，并通过 Wine 使用官方 Electron/N-API 运行时请求匿名包清单。

如自动获取失败，或需要测试指定版本，可手动覆盖：

~~~bash
./azurpromilia info \
  --installer /path/to/AzurPromilia_manjuu_installer.exe \
  --root "$HOME/Games/AzurPromilia"
~~~

只有实时官网发现、已有有效缓存和内置兜底都不可用时，普通流程才需要 `--installer`。

## 6. 查看服务器信息

第一次使用时指定一次游戏目录，程序会记住该路径：

~~~bash
./azp -p "$HOME/Games/AzurPromilia" status
~~~

之后可以直接运行：

~~~bash
./azp status
~~~

需要更换目录时可使用 `./azp path /new/path`，或再次传入 `-p/--path`。旧的 `--root` 参数仍兼容。

首次运行会调用 tools/bootstrap.py，需要 Python 3 和 Wine。官方元数据会缓存到游戏目录的 .azurpromilia/packageconfig.json，每次工作流默认刷新。

Status 默认显示：

- 游戏目录和安装状态；
- 本地版本与官方版本；
- 是否存在可用更新；
- 当前官方下载模式；
- 本地 manifest 的文件数与资源体积；
- 游戏启动入口是否存在；
- 官方预下载版本及本地预下载状态；
- 当前文件系统可用空间。

Status 不会执行全量 MD5 扫描；需要完整检查时使用 `./azp verify`。详细 package/CDN/launch metadata 仍可通过 `./azp info` 查看。

当前已实测的 SC000 版本使用 REALITY / packageType 2。

## 7. 首次下载和安装

先确认目标目录是你准备给本项目使用的目录。不要直接指向一个已有的 Windows 或其他启动器管理的游戏目录。

第一次安装：

~~~bash
./azp -p "$HOME/Games/AzurPromilia" install
~~~

目录会被记住，后续命令无需重复填写。需要调整并发数时可使用 `-j`：

~~~bash
./azp install -j 8
~~~

默认 --mode auto，会按官方元数据中的 downloadMode 选择包类型。只有确认自己要覆盖服务器默认模式时才使用 --mode reality 或 --mode slice。

下载完成后，程序会保存版本索引和本地 manifest。不要手工删除游戏目录中的 .azurpromilia、_index 等状态目录。

### 磁盘空间

已实测的 SC000 REALITY 安装传输了约 **84.84 GB** 资源。首次启动游戏客户端后，还需要额外下载约 **16.13 GB** 的客户端资源更新。请在游戏资源之外再预留空间，并为文件系统留出余量；实际需求会随官方版本变化。

SLICE 模式的当前样本包含 752 个分片，合并 ZIP 为 75,145,275,761 bytes。空目录安装时，分片下载与 ZIP 合并阶段的峰值至少约 **150.29 GB（140.0 GiB）**，还不包含解压空间。SLICE 全量安装尚未实测通过，不建议手动强制选择它。

## 8. 校验和修复

安装成功后执行完整校验：

~~~bash
./azp verify
~~~

当前版本的 REALITY 安装已实测 40,457 个文件全部通过大小和 MD5 校验。

如果同一个服务器版本的游戏文件缺失或损坏，可运行：

~~~bash
./azp sync
~~~

`sync` 是 `repair` 的别名；若希望使用原命令，`./azp repair` 完全等价。

Repair 会重新获取当前官方索引，只补齐缺失或损坏的资源。如果服务器已经发布新版本，Repair 会拒绝跨版本修复并提示使用 Update。

## 9. Steam / Proton 手动配置

下载器不会安装 Proton、创建 CompatData、添加 Steam 快捷方式或启动游戏。完成安装和 Verify 后，请按自己的 Steam 环境手动配置：

1. 在 Steam 中选择“添加非 Steam 游戏”，选择游戏目录下的 AzurPromilia.exe。
2. 将“起始位置/工作目录”设为游戏安装根目录。
3. 在“兼容性”中选择已安装的 Proton 兼容工具。
4. 将 tools/steam-launch-options.example 的内容复制到启动选项。
5. 将示例中的 YOUR_USER、游戏目录和 CompatData 路径替换成自己机器上的实际路径。

已验证的一套配置使用 dwproton-11.0-16、KDE Wayland、独立 CompatData 和 PROTON_ENABLE_WAYLAND=1。这只是一个实测配置，不保证适用于所有发行版、显卡和 Proton 版本。

完整说明见 [Proton 手动配置参考](PROTON_SETUP.md)。

## 10. 暂不建议用于正式目录的功能

代码中已经实现 preload 和 update，但尚未完成真实版本切换的完整验收。它们涉及新旧版本索引、预下载缓存复用和旧文件清理。

在专门的真实版本升级测试完成前：

- 不要把 preload / update 当作已稳定功能；
- 不要在唯一的正式游戏目录上试验；
- 先备份游戏目录和存档（如果存档位于游戏目录之外，也请确认其位置）；
- 不要手工将 _pre_download 内容覆盖到游戏目录。

## 11. 隐私与安全说明

- packageconfig.json 可能包含临时 CDN URL。不要公开上传或提交到 Git。
- 不要分享原版安装器中的 app_config.dat、密钥、账号 Cookie、token 或签名请求头。
- 本工具使用官方索引中的 MD5 和 ZIP CRC32 做资源完整性检查；它们不是数字签名。
- 本工具不处理账号登录，也不修改游戏反作弊或游戏客户端代码。
- 只使用官方来源的安装器和游戏资源。

## 12. 从源码构建

如不使用 Release 预编译包，需要 Go 1.22 或更新版本：

~~~bash
git clone https://github.com/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux.git
cd AzurPromilia_manjuu_installer_linux

go test -race ./...
go vet ./...
CGO_ENABLED=0 go build -trimpath -o ./azurpromilia ./cmd/azurpromilia

./azurpromilia help
~~~

运行时仍需 Python 3 和 Wine。官方 SC000 启动器会自动获取并缓存；如需手动覆盖可使用 `--installer`。请保留仓库中的 tools/ 目录与二进制相对位置。

## 13. 问题反馈

提交 Issue 时请附上：

- Linux 发行版和 CPU 架构；
- Wine 版本；
- 若使用了 `--installer` 手动覆盖，请提供该安装器版本及 SHA-256；
- 完整命令（可删除用户名和私人路径）；
- 完整错误日志与失败阶段。

不要直接上传原始 packageconfig.json、安装器内部配置、账号凭据或包含临时签名参数的请求记录。


## 14. 许可证

本项目使用 **PolyForm Noncommercial License 1.0.0**。

你可以为了非商业目的：

- 使用和研究代码；
- 修改、fork 和继续开发；
- 公开和再分发原版或修改版；
- 用于个人学习、实验、教育、公共研究和非营利项目。

商业用途不被许可。再分发时请同时保留许可证条款。完整法律文本见仓库根目录的 [LICENSE](../LICENSE)。
