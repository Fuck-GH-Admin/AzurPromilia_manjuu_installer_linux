# Azur Promilia Linux Downloader

**Manjuu / SC000 官方 PC 客户端的非官方 Linux 下载、安装与修复工具。**

本项目提供 Linux 原生 CLI：从你自己取得的官方 SC000 Windows 安装器中读取必要参数，通过官方接口获取匿名包清单，再从官方 CDN 下载游戏资源。它不分发游戏文件，不处理账号登录，也不自动安装或配置 Steam / Proton。


## 玩家从这里开始

- **[玩家安装指南](docs/PLAYER_GUIDE.md)**：依赖、下载、首次安装、校验、修复和问题反馈。
- **[Steam / Proton 手动配置](docs/PROTON_SETUP.md)**：已验证配置、启动选项说明和示例。
- **[GitHub Releases](https://github.com/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux/releases)**：下载预编译 Linux amd64 程序和 SHA256 校验文件。
- **[Actions 构建记录](https://github.com/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux/actions/workflows/build.yml)**：查看自动化测试和构建产物。

> **当前状态：Preview。** 已在 Manjuu / SC000 1.0.0.30 上完成 REALITY 全量安装、40,457 个资源文件校验、单文件修复和修复后复验；使用手动 Proton 配置成功进入游戏。SLICE 全量安装、Preload 和跨版本 Update 尚未完成真实环境验收。

## 已验证能力

| 功能 | 状态 |
|---|---|
| 官方 SC000 匿名元数据获取 | 已实测 |
| REALITY 首次安装 | 已实测通过 |
| 全量文件 Verify | 40,457 / 40,457 通过 |
| 单文件 Repair | 已实测通过 |
| Steam / Proton 手动配置 | 已成功进入并运行游戏 |
| SLICE 全量安装 | 待实测 |
| Preload / 跨版本 Update | 待实测 |
| Yostar 日本服 | 不支持，暂缓 |

当前实测版本的官方服务器选择 REALITY（逐文件）模式。程序默认按官方元数据选择下载模式；SLICE 虽已有实现，但完整安装还没有真实验收记录。请勿将尚未验收的 Preload / Update 用于唯一的正式游戏目录。

## 快速开始

1. 从 Manjuu 官方渠道获取 SC000 Windows 安装器 EXE。
2. 从 [Releases](https://github.com/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux/releases) 下载 Linux amd64 压缩包和 SHA256SUMS。
3. 阅读 [玩家安装指南](docs/PLAYER_GUIDE.md)，按步骤校验压缩包、运行 info 并安装。
4. 安装完成后运行 Verify；如需启动游戏，按 [Proton 手动配置指南](docs/PROTON_SETUP.md) 在 Steam 中配置。

Linux 运行时需要 Python 3 和 64 位 Wine，用于调用官方安装器携带的原版 Electron/N-API 组件获取匿名包清单。使用预编译 Release 时不需要安装 Go 或 Node.js。

**注意：目前尚未发布第一个 GitHub Release。** Release 工作流已经准备好；维护者发布版本标签或从 Actions 手动触发后，会生成正式的下载包。在此之前，可以从 [Actions 构建记录](https://github.com/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux/actions/workflows/build.yml) 选择最新成功的 main 分支运行，下载 artifact azurpromilia-linux：

1. 打开最新一次成功的 Linux build run，在 Artifacts 中下载 azurpromilia-linux。
2. 解压 Actions artifact，然后在其中运行：

~~~bash
sha256sum -c PACKAGE_SHA256SUMS
tar -xzf azurpromilia-linux-amd64.tar.gz
~~~

3. 进入解压出的目录，按 [玩家安装指南](docs/PLAYER_GUIDE.md) 操作。

Actions 构建产物有保留期限；长期下载请等待 Releases。

## 项目边界

- 只支持 Manjuu / SC000 渠道；Yostar 日本服暂缓。
- 下载器负责官方资源清单获取、下载、校验、安装和修复。
- 下载器不负责账号登录、游戏启动、Steam 快捷方式管理、Proton 安装或 CompatData 自动配置。
- 游戏首次进入后还可能自行下载约 16.13 GB 的客户端资源更新；这不属于下载器处理范围。
- `packageconfig.json` 可能包含临时 CDN URL，请勿公开分享。
- 资源 MD5 / ZIP CRC32 用于兼容官方资源完整性检查，不是数字签名。

## 开发与技术资料

玩家通常不需要阅读以下资料：

- [实现状态与未完成事项](docs/IMPLEMENTATION_STATUS.md)
- [本地测试与问题反馈记录](docs/LOCAL_TESTING.md)
- [SC000 实服验收记录](docs/NEXT_LIVE_TEST.md)
- [原版启动器流程逆向记录](docs/ORIGINAL_LAUNCHER_FLOW.md)
- [Yostar 未来兼容性调查（暂缓）](docs/YOSTAR_COMPAT_RESEARCH.md)

## 从源码构建

需要 Go 1.22 或更新版本：

~~~bash
git clone https://github.com/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux.git
cd AzurPromilia_manjuu_installer_linux

go test -race ./...
go vet ./...
CGO_ENABLED=0 go build -trimpath -o ./azurpromilia ./cmd/azurpromilia
~~~

运行时仍需 Python 3、64 位 Wine 和官方 SC000 安装器。请保留仓库中的 `tools/` 目录与二进制相对位置。

## License

仓库目前尚未提供 LICENSE 文件，代码再分发和二次使用的许可范围尚未声明。
