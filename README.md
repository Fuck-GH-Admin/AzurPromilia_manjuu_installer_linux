# Azur Promilia Linux Manager

Manjuu / SC000 官方 PC 客户端的非官方 Linux 下载、安装、校验与修复工具。

默认会自动从官方来源取得当前 SC000 启动器、获取匿名包清单并从官方 CDN 下载游戏资源。**玩家不需要自己寻找 Windows 安装器，也不需要手动准备 packageconfig。**

项目不分发游戏文件，不处理账号登录，也不自动配置 Steam / Proton。

## 当前状态

当前为 **Preview**。

已在 Manjuu / SC000 `1.0.0.30` 上实测：

- REALITY 全量安装成功；
- 40,457 / 40,457 个资源文件大小与 MD5 校验通过；
- 单文件损坏后的 Repair / Sync 成功；
- 修复后再次 Verify 全部通过；
- 手动配置 Proton 后已成功进入并运行游戏。

尚未完成真实环境验收：SLICE 全量安装、Preload、跨版本 Update。

Yostar 日本服当前不支持，相关研究暂缓。

## 下载

从 [GitHub Releases](https://github.com/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux/releases) 下载与你系统架构相符的 Linux 包和 `SHA256SUMS`：

- 普通 Intel / AMD Linux PC：`linux-amd64`
- AArch64 Linux：`linux-arm64`

例如：

~~~bash
package=azurpromilia-v0.1.0-preview.1-linux-amd64

sha256sum --ignore-missing -c SHA256SUMS
tar -xzf "$package.tar.gz"
cd "$package"

./azp --help
~~~

Release 包同时提供：

~~~text
azp              # 推荐的短命令
azurpromilia     # 完整命令，继续兼容
~~~

`azp` 是指向同一程序的 Linux 符号链接。

运行时需要：

- Python 3；
- 能运行官方 x86-64 Windows 启动器的 Wine 环境；
- 足够的磁盘空间和稳定网络。

amd64 已通过自动引导和 CDN 实测。arm64 下载器可以使用已有官方包清单和启动器文件；arm64 上的自动引导需要能运行 x86-64 Windows 程序的兼容环境。

## 交互菜单

在真实终端里直接运行：

~~~bash
./azp
~~~

会打开轻量交互菜单，可用 ↑/↓ 选择、Enter 确认、Esc 或 q 退出。菜单提供状态、版本 / 资源、安装、更新、校验、修复、预下载、游戏目录、高级选项、工具更新和帮助入口。

如果本地缓存的官方 metadata 已经显示服务器版本高于本地版本，主菜单会直接给出更新提示；为了避免每次打开菜单都偷偷启动 Wine，菜单本身不会主动刷新官方 metadata。运行 `status`、`resources`、`update` 等命令时会按现有工作流刷新。

“高级选项”可以保存默认下载并发数、默认安装包模式，并显式安装 Shell Completion。保存后的默认值仍可被命令行参数临时覆盖。

如果程序发现自己运行在管道、脚本或 CI 等非交互环境中，裸跑不会等待键盘输入，而是直接输出帮助。

## 快速开始

### 第一次使用

第一次只需要指定一次游戏目录：

~~~bash
./azp -p "$HOME/Games/AzurPromilia" install
~~~

程序会记住这个目录。配置保存在用户配置目录下，例如 Linux 通常为：

~~~text
~/.config/azurpromilia/config.json
~~~

之后不需要反复输入 `--root`。

### 日常命令

~~~bash
# 查看安装状态、本地/官方版本、更新状态、下载模式等
./azp status

# 浏览官方当前正式版 / 预下载版及可用资源模式
./azp resources
# 等价别名
./azp versions

# 只校验，不下载
./azp verify

# 校验并自动修复缺失 / 损坏文件
./azp sync

# 更新已有游戏安装
./azp update

# 更新 azp 工具自身（与游戏 update 是两个命令）
./azp upgrade

# 预下载下一版本（Preview）
./azp predownload
~~~

`sync` 是 `repair` 的更直观别名，`predownload` 是 `preload` 的别名。

需要更换游戏目录时：

~~~bash
./azp path "/mnt/games/AzurPromilia"
~~~

或在任意命令中重新指定：

~~~bash
./azp -p "/mnt/games/AzurPromilia" status
~~~

### Status 与 Info

`status` 面向普通玩家，只显示真正有用的摘要，例如：

~~~text
Azur Promilia Linux Manager

  工具版本:   v...
  游戏目录:   /home/user/Games/AzurPromilia
  安装状态:   已安装
  本地版本:   ...
  官方版本:   ...
  更新状态:   已是最新
  下载模式:   REALITY（逐文件）
  本地清单:   ... 个文件 · ... GiB
  启动入口:   AzurPromilia.exe · 已存在
  预下载:     无
  磁盘可用:   ... GiB
~~~

原来的 `info` 不删除，继续提供详细 package/CDN/launch metadata，主要用于排障和开发。

### 版本 / 资源选择

`azp resources`（别名 `azp versions`）会读取官方当前 metadata：

~~~bash
./azp resources
~~~

在真实终端中会进入交互选择：当前正式版可以按服务器推荐模式、REALITY 或 SLICE 安装；已有旧版本时会直接提供 Update；当前版本已安装时提供 Verify / Repair；存在预下载时则提供 Predownload。

在管道或脚本中不会进入交互，而只打印官方版本 / 资源概览表。

这里**不会伪造历史版本列表**：当前官方接口只暴露当前 Released 与当前 Preload，因此菜单也只展示这两类真实资源。



### Shell Completion

支持 Bash、Zsh 和 Fish。默认不会偷偷修改 shell 配置；需要时显式安装：

~~~bash
# 自动识别 $SHELL 并安装
./azp completion install

# 或明确指定 shell
./azp completion install bash
./azp completion install zsh
./azp completion install fish

# 只打印补全脚本，不写任何文件
./azp completion bash
~~~

安装后重新打开 shell（或重新加载 completion 系统）。之后可直接使用例如 `azp <Tab>`、`azp install --<Tab>`。

### 更新工具自身

`update` 始终表示**更新游戏**；`upgrade` 才表示**更新 azp 工具自身**：

~~~bash
# 只检查当前会选择哪个 GitHub Release
./azp upgrade --check

# 更新到最新发布（包括 Preview / pre-release）
./azp upgrade

# 只跟踪稳定版
./azp upgrade --stable
~~~

自更新仅针对 Release 包中的 Linux amd64 / arm64 版本。更新前会匹配架构、校验 GitHub Release asset 的 SHA-256（必要时回退到 `SHA256SUMS`）、安全解包并运行新二进制自检，之后才替换当前工具文件。源码或 Actions 的开发构建不会被静默替换；需要主动使用 `--force`。

### 操作完成后的下一步

安装、更新、修复、预下载和校验成功后，CLI 会给针对当前操作的下一步建议。例如安装后提示先 `azp verify` 再按 Proton 文档启动；修复后建议重新校验；预下载完成后明确提示等待正式发布并运行 `azp update`，不会让用户手工覆盖 `_pre_download`。

### 输出与进度

正常交互终端默认使用单行动态进度条，显示百分比、已传输大小、速度和文件进度。管道、脚本或 CI 中自动退化为周期性文本进度，不输出控制字符。

~~~bash
./azp install -v          # 详细模式：显示逐文件、重试、CDN 等诊断信息
./azp install -q          # 安静模式：只保留警告、错误和最终结果
./azp install --no-color  # 禁用 ANSI 颜色
./azp --version           # 查看工具版本
~~~

### 帮助

~~~bash
./azp --help
./azp install --help
./azp verify --help
./azp help repair
~~~

顶层帮助只展示普通玩家常用内容；`--installer`、`--packageconfig`、`--mode` 等排障参数放在对应子命令的高级选项中。

旧用法继续兼容：

~~~bash
./azurpromilia install --root "$HOME/Games/AzurPromilia" --workers 4
~~~

## 自动获取官方 SC000 启动器

普通玩家**不需要自己寻找或下载 Windows 启动器**。

第一次运行需要官方元数据的命令时，程序会：

1. 调用 Manjuu 官网当前使用的下载信息接口，获取 SC000 PC 启动器地址；
2. 下载并验证启动器结构，缓存到用户缓存目录；
3. 使用该启动器里的官方运行时取得匿名游戏包清单。

如果官网发现接口临时不可用，会退回项目内置的最后一个已验证官方版本。目前内置兜底为 **SC000 1.0.2.2**，SHA-256：

`0a1d1b97947838146c7b8f3f0d27dbba79e60372d043f60cd5401f2979531282`

已有安装器或需要排障时，仍可手动覆盖：

~~~bash
./azp status --installer /path/to/AzurPromilia_manjuu_installer.exe
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

./azurpromilia --help
~~~

源码构建默认生成完整命令名 `azurpromilia`；Release 包额外提供 `azp` 短别名。

运行时仍需 Python 3 和 64 位 Wine。请保留仓库中的 `tools/` 目录。

## License

本项目采用 [PolyForm Noncommercial License 1.0.0](LICENSE)。

允许非商业使用、研究、修改、fork 和再分发；**商业用途不被许可**。再分发时请保留许可证条款。
