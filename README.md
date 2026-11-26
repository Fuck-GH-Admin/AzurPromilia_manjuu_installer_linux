# AzurPromilia Manjuu Installer Linux

非官方 Linux 原生 CLI。Manjuu / SC000 正式版 1.0.0.30 已完成 REALITY 全量安装、40,457 个资源文件的 MD5 校验、单文件修复和修复后复验；客户端也已在 Linux + Proton 下成功进入游戏。

## 当前状态与本地测试

- [原版启动器安装、更新与预下载流程逆向记录](docs/ORIGINAL_LAUNCHER_FLOW.md)
- [实现状态、已知缺口与验收标准](docs/IMPLEMENTATION_STATUS.md)
- [本地测试步骤与问题反馈清单](docs/LOCAL_TESTING.md)
- [SC000 全量安装验收步骤与实测记录](docs/NEXT_LIVE_TEST.md)
- [未来计划：Yostar 日本启动器兼容性（暂缓）](docs/YOSTAR_COMPAT_RESEARCH.md)
- [SC000 Proton 配置与启动记录](docs/PROTON_SETUP.md)

当前为开发版。SLICE 全量安装、预下载和跨版本升级仍待实服验收。下载器不负责启动游戏或修改 Steam/Proton；安装后的手动配置见下文。

实测记录包含 HTTP 400 的对照结果、官方索引数据及 Range 续传的响应。[本机实测记录与直连命令](docs/LOCAL_TESTING.md)

## 构建

```sh
go build -trimpath -o azurpromilia ./cmd/azurpromilia
./azurpromilia help
```

GitHub Actions 在每次 push 时运行 Go 测试、vet 和 Linux amd64/arm64 构建，并上传二进制与 SHA256SUMS。

## 手动获取真实包清单

使用自己的原始安装器，在 Windows 或 Linux Wine 环境执行：

```sh
python3 tools/bootstrap.py --installer /path/to/original.exe --out packageconfig.json
```

引导器调用原版 Electron/N-API 组件生成匿名包清单请求，已在 Wine 10.0 和 `SC000` 样本上通过。它也会尝试获取游戏基础信息和 `/pre/check` 启动元数据；后两项若匿名接口不可用会作为可选信息跳过。下载器本身是 Linux 原生程序，也可直接导入自己取得的官方包清单。请将 `packageconfig.json` 保留在本机，其中可能有临时 CDN URL。

## 下载、安装与维护工作流

编译：

```sh
go build -trimpath -o azurpromilia ./cmd/azurpromilia
```

如果不传 `--packageconfig`，新工作流会调用 `tools/bootstrap.py` 获取最新匿名官方元数据。Linux 需要 Wine；Windows 可直接运行 Python 引导。元数据默认缓存在游戏目录的 `.azurpromilia/packageconfig.json`，每次工作流默认刷新；显式传入 `--packageconfig` 时使用指定文件。

```sh
# 查看服务端版本、下载模式、包类型和可用启动元数据
./azurpromilia info --installer /path/to/original.exe

# 首次安装：按服务端 downloadMode 自动选择 SLICE 或 REALITY
./azurpromilia install --installer /path/to/original.exe --root "$HOME/Games/AzurPromilia" --workers 4

# 预下载：对比 released/preload 索引，只下载变化文件
./azurpromilia preload --installer /path/to/original.exe --root "$HOME/Games/AzurPromilia" --workers 4

# 升级：复用通过 MD5 验证的预下载文件，再补齐、校验并清理旧版本文件
./azurpromilia update --installer /path/to/original.exe --root "$HOME/Games/AzurPromilia" --workers 4

# 修复：重新校验当前版本所有文件，只重下缺失或损坏资源
./azurpromilia repair --installer /path/to/original.exe --root "$HOME/Games/AzurPromilia" --workers 4

# 独立完整性校验
./azurpromilia verify --root "$HOME/Games/AzurPromilia"
```

可用 `--mode slice` 或 `--mode reality` 覆盖首次安装模式。默认 `auto` 使用原版枚举，`downloadMode=1` 为分片 ZIP，`downloadMode=2` 为逐文件。`info` 会显示服务器提供的游戏入口和启动参数。

旧的 `download` 命令仍保留，适合指定 packageconfig 的底层下载/探测。

## 实服探测

Linux 本机实测通过。Actions 中的 Official CDN live probe 是独立的手动验证入口，仍待运行。输入原始安装器的官方 HTTPS URL 后，工作流会核对样本 SHA-256，获取包清单，并下载和校验索引及一个不超过 64 MiB 的资源。

## 证据与限制

样本 SHA-256 为 `cf4b1f6233af2f2cb2d1c0de12f54d07606c3c84c829390b6caccfb2d0d1f04b`。内部 ZIP 位于偏移 12453620，包含 `resources/app.asar`、`res/app_config.dat` 和 Windows `cutils.node`。逐文件索引包含三列计数头、四列目录记录和五列文件记录。切片索引使用三列 ZIP 记录及五列切片记录。索引为 Base64 编码的 AES-CBC，先按 API 返回的大小和 MD5 验证密文。包清单签名由原版 Windows 原生模块处理。预下载和跨版本升级仍需实服测试。下载器的包清单引导使用匿名请求，账号登录由游戏客户端处理。

## Proton

下载器不会启动游戏或自动配置 Steam/Proton。REALITY 安装完成后，可按自己的环境手动添加 `AzurPromilia.exe` 为 Steam 非 Steam 游戏，选择兼容工具并配置独立前缀。已验证参数和可复制的启动选项见 [Proton 手动配置参考](docs/PROTON_SETUP.md) 与 [示例文件](tools/steam-launch-options.example)。
