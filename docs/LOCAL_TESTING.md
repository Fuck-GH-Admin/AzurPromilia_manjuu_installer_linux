# 本地测试与问题反馈

> 本文用于用户在自己的 Linux 机器上验收。优先使用独立测试目录，不要第一次就对已有正式游戏目录执行下载或覆盖。

## 1. 准备环境

- Linux x86-64（首轮建议）；
- Go 1.22 或更新版本；
- Python 3；运行下方的引导脚本语法检查时也需要 Node.js；
- 原始安装器 `AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe`；
- 若在 Linux 上运行一次性 metadata bootstrap，需要可用的 64 位 Wine 环境；也可以在 Windows x64 上运行 bootstrap；
- 足够的磁盘空间和稳定网络。

下载器本体不需要 Wine、Proton、Electron 或 Python 第三方包。Wine 只用于尝试调用原安装器附带的 Windows Electron/N-API 模块取得官方包清单。

## 2. 拉取、直连和编译

在同一个 Linux shell 中定义以下函数。通过 `direct` 启动的命令会清除本次执行的代理环境变量，覆盖包清单引导和 CDN 下载。Git 的直连配置仅作用于本仓库的 `origin`。

```bash
direct() {
  env -u http_proxy -u https_proxy -u all_proxy \
    -u HTTP_PROXY -u HTTPS_PROXY -u ALL_PROXY -u NODE_USE_ENV_PROXY "$@"
}

direct git -c http.proxy= clone https://github.com/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux.git
cd AzurPromilia_manjuu_installer_linux
git config --local remote.origin.proxy ''
git pull --ff-only

go version
go test -race ./...
go vet ./...
python3 -m py_compile tools/bootstrap.py
node --check tools/fetch_packageconfig.cjs

CGO_ENABLED=0 go build -trimpath -o ./azurpromilia ./cmd/azurpromilia
./azurpromilia help
```

如果编译或测试失败，请先记录完整错误，不要直接修改代码后只报告“现在能运行”。

本机通过直连完成官方包清单请求和 CDN 小文件探测，可以按上述方式运行。下载器支持 Go 的 `HTTP_PROXY`、`HTTPS_PROXY`、`NO_PROXY` 及其小写变量。需要使用现有代理配置时，可省略 `direct`。引导脚本目前没有独立的代理选项，代理环境下的实际行为待验证。

## 3. 获取官方包信息

在 Linux + Wine 环境：

```bash
direct python3 tools/bootstrap.py \
  --installer "$HOME/Downloads/AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe" \
  --out ./packageconfig.json
```

按实际位置替换安装器路径，中文桌面可能使用 `$HOME/下载`。如果 Wine 可执行文件不叫 `wine`，可传入 `--wine /path/to/wine`。无图形会话时，如果已安装 Xvfb 和 xauth，可用 `direct xvfb-run -a python3` 开头。在 Windows x64 环境运行 `python tools/bootstrap.py` 并使用相应的文件路径。

成功时应生成本地 `packageconfig.json`。该文件可能包含临时 CDN URL，不要提交到 Git、不要上传到公开 issue，也不要把账号 token、Cookie、签名头或密钥贴到聊天中。

如果失败，记录：
- Python 的完整错误；
- Wine 版本和架构（如使用 Wine）；
- 是否能启动原始安装器；
- 原生模块加载错误或官方 API 返回 code；
- 不要公开 `app_config.dat` 或 `secretKey`。

引导器发送匿名请求，`device-id` 请求头使用空值。已在本机验证 `SC000` 样本，账号登录和其他渠道仍待支持或验证。

## 4. 第一轮：只做真实服务器小文件探测

建议使用全新的空目录：

```bash
mkdir -p "$HOME/Games/AzurPromilia-probe"
direct ./azurpromilia download \
  --installer "$HOME/Downloads/AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe" \
  --packageconfig ./packageconfig.json \
  --root "$HOME/Games/AzurPromilia-probe" \
  --workers 2 \
  --probe
```

预期行为：

1. 从安装器读取 AES 参数；
2. 下载并校验官方索引文件；
3. 解密、解析索引；
4. 选择索引中最小的非空资源（不超过 64 MiB）；
5. 从 CDN 下载该资源并校验大小和 MD5；
6. 输出 `LIVE PROBE OK`。

`--probe` 把索引和探测资源保存在测试目录的 `.azurpromilia/`。默认按服务端 `downloadMode` 选包，可用 `--mode reality`、`--mode slice` 或兼容标志 `--slice` 显式选择。

如果失败，请记录从 `VERSION`、`RETRY`、`FAIL` 到最终 `ERROR` 的完整日志，以及 HTTP 状态码和响应错误。不要只截取最后一行。

## 5. 底层 `download` 命令：逐文件包下载（兼容入口）

只有 `--probe` 成功后才建议开始：

```bash
direct ./azurpromilia download \
  --installer "$HOME/Downloads/AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe" \
  --packageconfig ./packageconfig.json \
  --root "$HOME/Games/AzurPromilia-test" \
  --mode reality \
  --workers 4
```

下载完成后：

```bash
./azurpromilia verify --root "$HOME/Games/AzurPromilia-test"
```

本节显式选择 `packageType=2`。正常首次安装请使用第 11 节的 `install` 命令，完成版本标记和索引保存。

如需检查断点续传，可在下载一个较大的文件时按 Ctrl+C，然后原命令重新运行。观察是否发送 Range 请求并正确续传。若服务器忽略 Range 并返回 200，程序应从头覆盖 `.part`，不能把完整响应追加到旧分片。

## 6. 底层 `download` 命令：切片包下载（兼容入口）

```bash
direct ./azurpromilia download \
  --installer "$HOME/Downloads/AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe" \
  --packageconfig ./packageconfig.json \
  --root "$HOME/Games/AzurPromilia-slice-test" \
  --workers 4 \
  --slice
```

默认会合并 ZIP 并解压。随后：

```bash
./azurpromilia verify --root "$HOME/Games/AzurPromilia-slice-test"
```

如果只想测试切片下载和 ZIP 合并、不解压，可使用 `--no-extract`；此时 `verify --no-extract` 只验证切片和合并后的 ZIP，不检查解压目录。

## 7. 底层 `download --target preload`：原始暂存下载（兼容入口）

```bash
direct ./azurpromilia download \
  --installer "$HOME/Downloads/AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe" \
  --packageconfig ./packageconfig.json \
  --root "$HOME/Games/AzurPromilia-test" \
  --target preload
```

查看对应版本号后：

```bash
./azurpromilia verify \
  --root "$HOME/Games/AzurPromilia-test" \
  --target preload \
  --version VERSION_CODE
```

**不要手工把 `_pre_download` 目录覆盖进正式游戏目录。** 本节的底层 `download --target preload` 只负责按单份 packageconfig 下载暂存资源，不会比较 released/preload 差异。差异比较由新的 `preload` 命令完成；正式应用由 `update` 完成，且会重新按新版本索引验证 MD5。

## 8. 反馈时请提供什么

请按以下顺序反馈，便于定位：

1. Linux 发行版、架构、Go 版本、Wine 版本（如使用）；
2. 运行的完整命令（删除本机用户名等隐私路径即可）；
3. 完整错误日志；
4. 失败阶段：bootstrap、索引下载、索引解密、索引解析、资源下载、MD5 校验、ZIP 合并、ZIP 解压或最终 verify；
5. 如需分享 `packageconfig.json`，先删除 UID、token、Cookie、签名等字段，并把 CDN URL 的路径和查询参数删掉，只保留必要的字段名、packageType、版本号、文件大小和域名。通常只需要贴 `code`、`versionCode`、`versionName`、`downloadMode`、`packageType`、`fileSize`、`fileMd5` 和 CDN 主机名。

不要上传原始 `packageconfig.json`、`app_config.dat`、`config.json`、`secretKey`、账号 token、Cookie 或任何个人设备标识。

## 9. Proton（仅提供手动配置说明）

下载器不启动游戏、不安装 Proton、不修改 Steam，也不创建 CompatData。资源安装和 `verify` 完成后，用户可按自己的 Steam/Proton 环境手动添加游戏入口。

本机 SC000 的已验证参数、Steam 设置步骤和可复制的启动选项示例见 [Proton 手动配置参考](PROTON_SETUP.md) 与 [steam-launch-options.example](../tools/steam-launch-options.example)。游戏客户端首次进入后还需要约 16.13 GB 的额外资源更新空间。

## 10. 本机实测记录（2026-11-25）

- Ubuntu 26.04.1 x86-64、Go 1.27.0、Python 3.14.4、Node.js 22.23.4、Wine 10.0。Go race 测试、vet、Python/Node 语法检查和 Linux amd64 构建均通过。
- 原始安装器的 SHA-256 与仓库记录一致。Go CLI 能提取索引参数。使用本地 HTTP 服务和测试数据运行了索引解密、`--probe`、逐文件下载、`verify` 和重复下载跳过已校验资源。
- GitHub 拉取和以下官方 API、CDN 请求均使用直连，保留 TLS 校验。

### 匿名包清单

修正 Electron 的 ASAR 读取后，在独立 Wine 前缀中对照原版请求头逐项测试。

| 请求头变化 | API 主站 | API 备用站 |
|---|---|---|
| 原引导请求 | HTTP 400，JSON `Bad Request`，无业务码 | HTTP 400，同样的错误 |
| 仅增加空 `grp-trace-id` | HTTP 400 | 未单测 |
| 仅增加空 `device-id` | HTTP 200，`code=0` | HTTP 200，`code=0` |
| 两个头均为空 | HTTP 200，`code=0` | 未单测 |

API 主站为 `api-grp.manjuu.com`，备用站为 `api-grp-l2.manjuu.com`。引导器增加空 `device-id` 后，正常运行 `tools/bootstrap.py` 生成了包清单。该响应提供正式版 `versionCode=1.0.0.30`（`versionName=1.0.0`）的切片包和逐文件包。

### 官方 CDN

逐文件包的第一条记录是三列计数 `40486,40457,29`，对应文件与目录的总数、文件数和目录数。原版逐文件解析器跳过该记录。Linux 解析器修复后接收这种计数头，继续校验并解析后续目录、文件记录。

| 包类型 | 索引大小 | 解析的资源 | 探测资源大小 | 结果 |
|---|---:|---:|---:|---|
| 逐文件包（`packageType=2`） | 15,024,044 字节 | 40,457 个文件，29 个目录 | 4 字节 | `LIVE PROBE OK` |
| 切片包（`packageType=1`） | 148,184 字节 | 752 个分片 | 45,275,761 字节 | `LIVE PROBE OK` |

主 CDN `syncstation.manjuu.com` 和备用 CDN `l2-syncstation.manjuu.com` 分别在新的测试目录中通过两种探测。每次均完成索引下载、MD5 与大小校验、解密、解析，以及资源的 MD5 与大小校验。

用已验证分片的前 42,991,616 字节创建 `.part` 后，下载器发送 `Range: bytes=42991616-`，主 CDN 返回 `206` 和 `Content-Range: bytes 42991616-45275760/45275761`，续传后的文件通过校验。

后续 SC000 REALITY 全量安装见 [2026-11-26 实测记录](NEXT_LIVE_TEST.md)。

## 11. 完整工作流命令

本节对应 `install`、`info`、`preload`、`update`、`repair`。SC000 在官方 REALITY 模式下完成了全量下载、安装与校验。首次测试请使用独立目录。

### 自动获取当前官方元数据

不传 `--packageconfig` 时，程序会自动调用 `tools/bootstrap.py`，把 packageconfig 缓存到游戏目录的 `.azurpromilia/packageconfig.json`。默认每次刷新，避免拿旧版本清单执行 update。

```bash
./azurpromilia info \
  --installer "$HOME/Downloads/AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe"
```

Linux 需要 Python 3 和可运行原版 N-API 模块的 Wine。若二进制放在 GitHub Actions 的 `dist/` 目录中，需保留同目录下的 `tools/` 文件夹。若要离线重用已有 JSON，可显式传入 `--packageconfig ./packageconfig.json`。

### 首次安装：自动选择服务器模式

```bash
./azurpromilia install \
  --installer "$HOME/Downloads/AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe" \
  --root "$HOME/Games/AzurPromilia-test" \
  --workers 4
```

默认 `--mode auto`：原版 `downloadMode=1` 选择切片包，`downloadMode=2` 选择逐文件包。可用 `--mode slice` 或 `--mode reality` 手动覆盖。若目录已有 `_index/version`，install 会拒绝覆盖，应该使用 update 或 repair。

### 预下载与更新

先确认本机已安装版本，再运行：

```bash
./azurpromilia preload \
  --installer "$HOME/Downloads/AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe" \
  --root "$HOME/Games/AzurPromilia-test" \
  --workers 4

./azurpromilia verify \
  --root "$HOME/Games/AzurPromilia-test" \
  --target preload \
  --version PRELOAD_VERSION_CODE
```

preload 通过当前 released 与 preload 两份 packageType 2 索引按路径和 MD5 做差异比较，只把新增或变化文件放进 `_pre_download/<version>`。

版本正式切换后：

```bash
./azurpromilia update \
  --installer "$HOME/Downloads/AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe" \
  --root "$HOME/Games/AzurPromilia-test" \
  --workers 4

./azurpromilia verify --root "$HOME/Games/AzurPromilia-test"
```

update 会对预下载缓存重新计算 MD5，复用匹配新版本索引的文件，然后下载其余缺失或不匹配资源。只有全部文件下载和校验成功后才清理旧 manifest 追踪的文件并更新版本标记。

更新采用逐文件原子替换，不是整次更新的全局事务回滚。中途失败时，部分新版本文件可能已落盘，但版本标记尚未前移；修复网络后重新执行 update 会继续检查并收敛。请勿手工删除 `.azurpromilia`、`_index` 或 `_pre_download`。

### 修复

```bash
./azurpromilia repair \
  --installer "$HOME/Downloads/AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe" \
  --root "$HOME/Games/AzurPromilia-test" \
  --workers 4
```

repair 只对当前服务端版本逐文件检查大小和 MD5，不清理旧版本文件。若服务器已发布新版本，应先执行 update。

### 首轮完整测试建议

1. 先运行 `info`，确认 version、downloadMode 和包类型。
2. 在空目录运行 `install`，确认实际选中的 packageType 与服务器 downloadMode 一致。
3. 完成后运行 `verify`，检查是否所有 manifest 文件均通过。
4. 再运行一次 `verify`，确认重复校验稳定。
5. 若服务器提供 preload，运行 preload 并校验暂存文件；待正式版本切换后再运行 update。
6. 对一个可丢弃的测试目录运行 repair，确认损坏文件会重新下载。
7. 全部完成后再考虑正式游戏目录。

SLICE 全量安装、跨版本更新和其他渠道仍待验收。空间预检查、ZIP 处理和下载失败处理见第 12 节。使用 Proton 时可按第 9 节启动游戏。


## 12. 第一阶段：下载可靠性改动

当前分支新增以下保护。GitHub Actions Run #139（`45620110972`）已通过 `go test -race ./...`、`go vet ./...`、Python/Node 检查、Linux amd64/arm64 构建和 Artifact 上传：

- `install`、`repair`、`preload`、`update` 和底层 `download` 在正式派发资源任务前，会校验已有文件并计算缺失/损坏文件的预计下载字节数。
- 如果文件系统可用空间不足，会在下载开始前报出所需空间和当前可用空间。估算会按完整文件大小计算缺失/损坏资源；只有 `.part` 本身完整通过大小和 MD5 校验时才会直接复用，普通未完成分片仍按完整文件大小保守估算。
- SLICE 模式在开始下载前，把缺失分片的预计下载字节数与后续合并 ZIP 的完整大小合并预检；如果已有通过 MD5 校验的 ZIP，则复用它并跳过分片下载。当前正式版 752 个分片的 ZIP 为 75,145,275,761 bytes，全部分片均缺失时，下载 + 合并至少需要 150,290,551,522 bytes（约 150.29 GB / 140.0 GiB），尚不含解压空间。合并前仍会核对分片大小总和与官方 ZIP 声明，解压前再根据 ZIP 中央目录统计缺失/损坏文件的解压空间。
- ZIP 会拒绝重复路径、文件/目录路径冲突、路径穿越、符号链接和特殊文件；累计未压缩大小时检查溢出，并将每个条目的实际解压流限制在声明大小加 1 字节以内。
- 下载中的单个文件在既定重试全部失败后，会停止继续派发新文件，并取消其他正在执行的下载请求。明确的 HTTP 4xx 永久错误（408、425、429 除外）会立即切换下一个 CDN。`verify` 不会因此提前停止，会继续扫描清单以汇总问题。
- 磁盘空间检查基于文件系统当时的可用块数，不是预留空间；其他程序同时写盘或用户配额仍可能导致后续写入失败。

自动化回归：

```bash
go test -race ./...
go vet ./...
```

官方 CDN 的全量下载结果记录在 [SC000 实服验收](NEXT_LIVE_TEST.md)。


## 13. Actions 公网能力确认与凌晨测试边界（2026-11-26）

GitHub hosted Windows runner 已确认可以访问官方网络，不需要把“Actions 无公网”当作风险项：

- 日本官网和 `PCdownload`：HTTP 200。
- `api-grp.manjuu.com`、`api-grp-l2.manjuu.com`：根路径 HTTP 404，但连接、TLS 和 HTTP 请求均成功。
- Actions 可从 `launcher-pkg-azupro-jp.yo-star.com` 下载当前日本官方安装器；本次实测 179,893,048 字节。
- 当前日本包 SHA-256：`dde29b4fa4a6eabfb664838c807460e65746f590f1a51754a33d976763bb1e3f`。
- 该日本包不是现有 Manjuu/SC000 bootstrap 使用的内嵌 ZIP 封装，运行 `tools/bootstrap.py` 会明确失败为 `embedded launcher ZIP not found`。

对应 Actions：

- 网络与新 SHA 发现：Run #3 / `45660232078`
- 新安装器结构验证：Run #5 / `45660583827`

本机 Manjuu / SC000 的全量测试使用原始 SC000 安装器。有真实 preload 和新正式版本时，再验证 `preload -> update -> verify`。


## 13. SLICE 全链路空间预检查

SLICE 安装会在开始传输前同时计算缺失分片和合并 ZIP 的空间需求，避免先下载所有分片、再因无法保存 ZIP 而失败。普通安装路径若已有有效 ZIP 可直接复用；底层 `download --no-extract` 会保留分片，以便 `verify --no-extract` 检查切片和 ZIP。当前 SC000 正式版的分片总量与 ZIP 声明均为 75,145,275,761 bytes；从空目录安装时，下载与合并的最低峰值约 150.29 GB（140.0 GiB）。

解压所需空间无法仅从切片索引得知，程序会在下载并合并 ZIP 后读取中央目录，再次检查解压阶段的额外空间。所有检查均基于当时文件系统可用空间，不是磁盘预留；请避免测试期间其他进程大量写盘。
