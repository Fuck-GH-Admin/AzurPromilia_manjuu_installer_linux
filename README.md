# AzurPromilia Manjuu Installer Linux

非官方 Linux 原生 CLI。本机已通过官方包清单获取、两种包类型的 CDN 小文件探测和切片续传测试。当前为开发版，全量安装和游戏启动仍待验证。

## 当前状态与本地测试

- [实现状态、已知缺口与验收标准](docs/IMPLEMENTATION_STATUS.md)
- [本地测试步骤与问题反馈清单](docs/LOCAL_TESTING.md)

请先阅读实现状态、原版流程和本地测试文档。最新分支正在补齐自动安装、预下载、升级与修复链路；全量实服验收仍需本地完成。

实测记录包含 HTTP 400 的对照结果、官方索引数据及 Range 续传的响应。[本机实测记录与直连命令](docs/LOCAL_TESTING.md)

## 构建

```sh
go build -trimpath -o azurpromilia ./cmd/azurpromilia
./azurpromilia help
```

GitHub Actions 在每次 push 时运行 Go 测试、vet 和 Linux amd64/arm64 构建，并上传二进制与 SHA256SUMS。

## 获取真实包清单（实验性一次性引导）

使用自己的原始安装器，在 Windows 或 Linux Wine 环境执行：

```sh
python3 tools/bootstrap.py --installer /path/to/original.exe --out packageconfig.json
```

引导器调用原版 Electron/N-API 组件生成匿名包清单请求，已在 Wine 10.0 和 `SC000` 样本上通过。若官方接口拒绝匿名请求，请停止并记录错误。下载器本身是 Linux 原生程序，也可直接导入自己取得的官方包清单。请将 `packageconfig.json` 保留在本机，其中可能有临时 CDN URL。

## 完整工作流（开发中）

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

可用 `--mode slice` 或 `--mode reality` 覆盖首次安装模式；默认 `auto` 使用原版枚举：`downloadMode=1` 为分片 ZIP，`downloadMode=2` 为逐文件。新工作流目前不启动游戏，不修改 Proton；`info` 会显示服务器可提供的 launch metadata，供手动配置使用。

旧的 `download` 命令仍保留，适合指定 packageconfig 的底层下载/探测。

## 实服探测

Linux 本机实测通过。Actions 中的 Official CDN live probe 是独立的手动验证入口，仍待运行。输入原始安装器的官方 HTTPS URL 后，工作流会核对样本 SHA-256，获取包清单，并下载和校验索引及一个不超过 64 MiB 的资源。

## 证据与限制

样本 SHA-256 为 `cf4b1f6233af2f2cb2d1c0de12f54d07606c3c84c829390b6caccfb2d0d1f04b`。内部 ZIP 位于偏移 12453620，包含 `resources/app.asar`、`res/app_config.dat` 和 Windows `cutils.node`。逐文件索引包含三列计数头、四列目录记录和五列文件记录。切片索引使用三列 ZIP 记录及五列切片记录。索引为 Base64 编码的 AES-CBC，先按 API 返回的大小和 MD5 验证密文。包清单签名由原版 Windows 原生模块处理。自动版本发现、预下载差异比较与合并、旧文件清理仍待实现。

## Proton

Steam 添加实际游戏 EXE 为非 Steam 游戏，设置工作目录为游戏目录，兼容性选择 Proton/GE-Proton。可用 STEAM_COMPAT_DATA_PATH="$HOME/Games/compatdata/azurpromilia" %command% 隔离前缀。实际游戏 EXE 和运行兼容性待实测；不修改反作弊或游戏代码。
