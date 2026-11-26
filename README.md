# Azur Promilia Linux Downloader

Manjuu / SC000 官方 PC 客户端的非官方 Linux 下载、安装与修复工具。

项目从你自己取得的官方 SC000 Windows 安装器中读取必要参数，通过官方接口获取匿名包清单，再从官方 CDN 下载游戏资源。**不分发游戏文件，不处理账号登录，也不自动配置 Steam / Proton。**

## 当前状态

当前为 **Preview**。

已在 Manjuu / SC000 `1.0.0.30` 上实测：

- REALITY 全量安装成功；
- 40,457 / 40,457 个资源文件大小与 MD5 校验通过；
- 单文件损坏后的 Repair 成功；
- 修复后再次 Verify 全部通过；
- 手动配置 Proton 后已成功进入并运行游戏。

尚未完成真实环境验收：

- SLICE 全量安装；
- Preload；
- 跨版本 Update。

Yostar 日本服当前不支持，相关研究暂缓。

## 下载

### GitHub Release

正式发布后，请优先从 [Releases](https://github.com/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux/releases) 下载 Linux amd64 压缩包和 SHA256 校验文件。

### 当前 Preview 构建

在第一个 Release 发布前，可以从 [Actions](https://github.com/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux/actions/workflows/build.yml) 选择最新成功的 **main** 构建，下载 artifact `azurpromilia-linux`。

解压 artifact 后：

~~~bash
sha256sum -c PACKAGE_SHA256SUMS
tar -xzf azurpromilia-linux-amd64.tar.gz
cd azurpromilia-linux-amd64
./azurpromilia help
~~~

预编译包运行时需要：

- Linux x86-64；
- Python 3；
- 64 位 Wine；
- 你自己从官方渠道取得的 Manjuu / SC000 Windows 安装器。

不需要安装 Go 或 Node.js。

## 快速开始

~~~bash
GAME_DIR="$HOME/Games/AzurPromilia"
INSTALLER="$HOME/Downloads/AzurPromilia_manjuu_installer.exe"

# 查看当前官方版本和下载模式
./azurpromilia info \
  --installer "$INSTALLER" \
  --root "$GAME_DIR"

# 首次安装
./azurpromilia install \
  --installer "$INSTALLER" \
  --root "$GAME_DIR" \
  --workers 4

# 完整校验
./azurpromilia verify \
  --root "$GAME_DIR"

# 同版本文件损坏时修复
./azurpromilia repair \
  --installer "$INSTALLER" \
  --root "$GAME_DIR" \
  --workers 4
~~~

默认使用 `--mode auto`，按官方返回的 `downloadMode` 选择包类型。当前实测服务器选择 REALITY。

完整说明见 [玩家使用指南](docs/PLAYER_GUIDE.md)。

## 游戏启动

下载器只负责资源下载、安装、校验和修复。

Steam / Proton 需要用户自行配置。已验证的一套配置和启动选项示例见：

- [Proton 手动配置](docs/PROTON_SETUP.md)
- [Steam 启动选项示例](tools/steam-launch-options.example)

首次进入客户端后，游戏自身还可能继续下载额外资源；这部分不由本项目管理。

## 注意事项

- 不建议在唯一的正式游戏目录中测试尚未验收的 Preload / Update。
- `packageconfig.json` 可能包含临时 CDN URL，不要公开分享。
- 本工具使用官方资源清单中的 MD5 / ZIP CRC32 做完整性检查，它们不是数字签名。
- 只使用官方来源的安装器和游戏资源。

## 开发资料

- [玩家使用指南](docs/PLAYER_GUIDE.md)
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

运行时仍需 Python 3、64 位 Wine 和官方 SC000 安装器。请保留仓库中的 `tools/` 目录。

## License

本项目采用 [PolyForm Noncommercial License 1.0.0](LICENSE)。

允许非商业使用、研究、修改、fork 和再分发；**商业用途不被许可**。再分发时请保留许可证条款。
