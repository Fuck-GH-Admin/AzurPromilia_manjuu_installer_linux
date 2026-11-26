# Azur Promilia Linux Downloader

Manjuu / SC000 官方 PC 客户端的非官方 Linux 下载、安装与修复工具。

默认会自动从官方来源取得当前 SC000 启动器、获取匿名包清单并从官方 CDN 下载游戏资源。**玩家不需要自己寻找 Windows 安装器。** 项目不分发游戏文件，不处理账号登录，也不自动配置 Steam / Proton。

## 当前状态

当前为 **Preview**。

已在 Manjuu / SC000 `1.0.0.30` 上实测：

- REALITY 全量安装成功；
- 40,457 / 40,457 个资源文件大小与 MD5 校验通过；
- 单文件损坏后的 Repair 成功；
- 修复后再次 Verify 全部通过；
- 手动配置 Proton 后已成功进入并运行游戏。

尚未完成真实环境验收：SLICE 全量安装、Preload、跨版本 Update。

Yostar 日本服当前不支持，相关研究暂缓。

## 下载

从 Release 选择与系统架构匹配的 Linux 包，同时下载 `SHA256SUMS`。`x86_64` 选 amd64，`aarch64` 选 arm64。[GitHub Releases](https://github.com/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux/releases)

以下用第一个预览版的 amd64 包举例：

~~~bash
package=azurpromilia-v0.1.0-preview.1-linux-amd64
sha256sum --ignore-missing -c SHA256SUMS
tar -xzf "$package.tar.gz"
cd "$package"
./azurpromilia help
~~~

自动获取官方包清单需要：

- Python 3 和可运行官方 x86-64 Windows 启动器的 Wine 环境
- 足够的磁盘空间和网络

amd64 已通过自动引导和 CDN 实测。arm64 下载器可使用已有官方包清单和启动器文件；arm64 上的自动引导需具备运行 x86-64 Windows 程序的环境。

## 快速开始

~~~bash
GAME_DIR="$HOME/Games/AzurPromilia"

# 查看当前官方版本和下载模式
./azurpromilia info --root "$GAME_DIR"

# 首次安装
./azurpromilia install \
  --root "$GAME_DIR" \
  --workers 4

# 完整校验
./azurpromilia verify --root "$GAME_DIR"

# 同版本文件损坏时修复
./azurpromilia repair \
  --root "$GAME_DIR" \
  --workers 4
~~~

第一次运行需要官方元数据的命令时，程序会：

1. 调用 Manjuu 官网当前使用的下载信息接口，获取 SC000 PC 启动器地址；
2. 下载并验证启动器结构，缓存到用户缓存目录；
3. 使用该启动器里的官方运行时取得匿名游戏包清单。

如果官网发现接口临时不可用，会退回项目内置的最后一个已验证官方版本。目前内置兜底为 **SC000 1.0.2.2**，SHA-256：

`0a1d1b97947838146c7b8f3f0d27dbba79e60372d043f60cd5401f2979531282`

已有安装器或需要排障时，仍可手动覆盖：

~~~bash
./azurpromilia info \
  --installer /path/to/AzurPromilia_manjuu_installer.exe \
  --root "$GAME_DIR"
~~~

默认使用 `--mode auto`，按官方返回的 `downloadMode` 选择包类型。当前实测服务器选择 REALITY。

完整说明见 [玩家使用指南](docs/PLAYER_GUIDE.md)。

## 游戏启动

下载器只负责资源下载、安装、校验和修复。Steam / Proton 由用户手动配置：

- [Proton 手动配置](docs/PROTON_SETUP.md)
- [Steam 启动选项示例](tools/steam-launch-options.example)

首次进入客户端后，游戏自身还可能继续下载额外资源；这部分不由本项目管理。

## 注意事项

- 不建议在唯一的正式游戏目录中测试尚未验收的 Preload / Update。
- `packageconfig.json` 可能包含临时 CDN URL，不要公开分享。
- 本工具使用官方资源清单中的 MD5 / ZIP CRC32 做完整性检查，它们不是数字签名。
- 自动获取的启动器只用于读取官方参数和运行元数据引导，不会安装 Windows 启动器。

## 开发资料

- [玩家使用指南](docs/PLAYER_GUIDE.md)
- [预览版发布验证报告](docs/RELEASE_VALIDATION.md)
- [实现状态](docs/IMPLEMENTATION_STATUS.md)
- [本地测试记录](docs/LOCAL_TESTING.md)
- [SC000 实服验收记录](docs/NEXT_LIVE_TEST.md)
- [原版启动器流程](docs/ORIGINAL_LAUNCHER_FLOW.md)

## 从源码构建

需要 Go 1.22 或更新版本：

~~~bash
git clone https://github.com/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux.git
cd AzurPromilia_manjuu_installer_linux

go test -race ./...
go vet ./...
CGO_ENABLED=0 go build -trimpath -o ./azurpromilia ./cmd/azurpromilia
~~~

运行时仍需 Python 3 和 64 位 Wine。请保留仓库中的 `tools/` 目录。

## License

本项目采用 [PolyForm Noncommercial License 1.0.0](LICENSE)。

允许非商业使用、研究、修改、fork 和再分发；**商业用途不被许可**。再分发时请保留许可证条款。
