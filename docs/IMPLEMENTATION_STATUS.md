# 当前实现状态与未完成事项

> 更新日期：2026-11-25  
> 状态：开发版 / 本机实服探测通过，全量安装待验收。

本文以仓库当前 Go 源码、工作流和用户提供的 Windows 安装器 `AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe` 为依据。区分“源码已经实现”“原始 EXE 中已确认”“仍需真实环境验证”，不把单元测试或本地 HTTP 测试当成官方服务端证据。

## 1. 样本与原版行为中已经确认的内容

样本 SHA-256：

`cf4b1f6233af2f2cb2d1c0de12f54d07606c3c84c829390b6caccfb2d0d1f04b`

样本是 Windows PE32+ x86-64 安装器。其内部 ZIP 包含 Electron ASAR、`res/app_config.dat` 和 Windows 原生模块 `cutils.node`。原版关键下载逻辑位于 ASAR 的 `main.js`。

从原版代码可确认：

- 游戏包信息接口：`GET /pc-launcher/client/gameversion/packageconfig`。
- 原版配置中的 API 域名：`https://api-grp.manjuu.com`、`https://api-grp-l2.manjuu.com`。
- 请求参数包含 `launcherId`、`uid`。
- 原版 GET 签名调用 `cutils.extApi3(timestamp + sortedQueryString + utoken)`；查询参数按键名排序。`utoken` 来自原版登录态，匿名状态下为空。
- 原版请求头包含 `device-id`、`grp-trace-id`、`grp-app-id`、`grp-platform`、`grp-channel-code`、`grp-sub-channel-code`、`sdk-version`、`channel-sdk-version`、`grp-launcher-version-code`、`lang` 和 `grp-client-info`。
- 原版平台字段为 `Launcher`；默认子渠道信息包含 `SC000`，但运行时 `delyinfo.json` 可以提供实际子渠道和 trace ID。
- 原版返回数据包含正式版本 `released`、预下载 `preload`、`gamePackageList`、`domainList`，以及下载模式等信息。
- 原版根据下载模式使用两类包：`packageType=2` 的逐文件资源包，或 `packageType=1` 的切片 ZIP 包。
- 逐文件索引：实测样本首行包含 3 列计数，原版跳过这一行。4 列记录表示目录，5 列记录表示文件，文件记录字段包含大小、MD5、相对文件名和远程路径。
- 切片索引：3 列记录表示合并后 `game.zip` 的大小和 MD5；5 列记录表示 ZIP 分片，原版将第 3 列作为分片大小、第 4 列同时用作分片 MD5 和本地分片名、第 5 列作为远程路径。
- 索引正文是 Base64 编码的 AES-CBC 密文；原版使用内嵌的 AES 参数解密。
- 原版资源校验以文件大小和 MD5 为准；切片模式还会合并 ZIP、解压并执行后续校验。
- 原版更新流程还涉及旧版本索引、排除文件列表、预下载版本检查及预下载文件合并。

## 2. 当前仓库已经写入的功能

以下表示代码中已有对应实现，不代表已经通过官方 CDN 的完整验收：

| 项目 | 当前实现 |
|---|---|
| Linux CLI | Go 单文件程序，无 GUI |
| 安装器索引参数读取 | 从原始 EXE 内部 ASAR 提取 AES 参数 |
| 索引密文下载 | 使用包信息中的 URL、大小和 MD5 |
| 索引解密 | Base64 + AES-CBC + PKCS#7 去填充 |
| 逐文件索引解析 | 可选 3 列计数头、4 列目录记录、5 列文件记录 |
| 切片索引解析 | 3 列 ZIP 元数据、5 列分片记录 |
| 文件下载 | HTTP GET、并发 worker、临时 `.part` 文件 |
| 断点续传 | 根据现有 `.part` 长度请求 HTTP Range；遇到服务器返回完整 200 时退回重下 |
| CDN 重试 | 多个 CDN URL 时轮换尝试并重试 |
| 完整性校验 | 下载落位前校验文件大小和 MD5 |
| 切片处理 | 按索引顺序合并分片、校验最终 ZIP 的大小和 MD5 |
| ZIP 解压 | 解压到目标目录，拒绝路径穿越和 ZIP 符号链接条目 |
| 解压结果检查 | 根据 ZIP 中央目录的 CRC32 和解压后大小检查文件 |
| 本地清单 | 将解析后的资源清单保存在 `.azurpromilia/` 下，供后续 `verify` 使用 |
| 日志 | 文件进度、传输速度、失败项、重试和汇总 |
| GitHub Actions | Go 测试、vet、Python/Node 语法检查、Linux amd64/arm64 构建和构建产物上传 |
| 实服探测 | 本机已从官方 CDN 完成两种包类型的探测；Windows 手动工作流待验证 |

### 当前命令行为

- `download` 默认选择 `packageType=2`；传入 `--slice` 才选择 `packageType=1`。
- `--target preload` 将预下载资源放入暂存目录，不会自动覆盖正式游戏文件。
- `--probe` 先下载并解密索引，再从索引中选择最小的非空资源（上限 64 MiB）下载校验；它不会执行完整安装。
- `verify` 只依据本地保存的 manifest 检查已知文件，不会重新请求服务器。
- 切片模式的 `verify` 会检查分片、合并 ZIP；默认还会检查已解压文件的 CRC32 和大小。使用 `--no-extract` 时跳过解压目录检查。
- `verify --target preload --version VERSION_CODE` 检查预下载暂存目录。
- 当前没有 `apply-preload` 子命令。

## 3. 尚未完成的部分

### P0：官方 API 获取流程仍属实验性

下载器目前没有纯 Go 官方 API 客户端。`tools/bootstrap.py` 和 `tools/fetch_packageconfig.cjs` 是一次性引导方案：从用户自己的安装器提取原版 Electron 和 `cutils.node`，再尝试用原版原生模块签名并请求包信息。

本机在 Wine 10.0 和 `SC000` 样本上完成了匿名 API 验收。两个官方 API 域名都接受带空 `device-id` 请求头的请求，返回 `code=0` 和正式版包清单。Windows、其他 Wine 版本、其他渠道及需要登录的流程仍待验证。[本机实测记录](LOCAL_TESTING.md#10-本机实测记录2026-11-25)

因此，当前 `packageconfig.json` 必须视为本地临时输入。若引导脚本返回非零 API code、缺少 package、或下载器不能解密索引，应保留错误日志，不要猜测签名参数或伪造服务器响应。

### P0：完整下载尚未经过官方 CDN 验收

本机两种包的 `--probe` 均从官方 CDN 完成索引下载、大小和 MD5 校验、解密、解析及一个资源的下载和校验。使用预置 `.part` 已验证切片续传。全量下载和最终 `verify`、切片合并与解压、游戏启动仍待实测。[官方 CDN 实测记录](LOCAL_TESTING.md#官方-cdn)

### P1：服务器下载模式和排除规则尚未自动兼容

- 当前通过 `--slice` 手动选择 `packageType=1`，不读取服务器 `downloadMode` 自动决定包类型。
- 当前没有完整解析和执行 `gameExcludedFileList`。
- 不会自动根据已安装游戏版本、旧索引和服务端版本选择 install / upgrade / repair 路径。
- 逐文件和切片均已通过真实索引探测，两种模式的完整安装仍待验收。

### P1：预下载不是完整的更新补丁流程

当前 `--target preload` 只是下载预下载包索引所列资源到暂存目录。尚未实现原版的：

- `/pre/check` 等预下载状态与版本检查；
- 当前正式版本与预下载版本的差异比较；
- 正式索引与预下载索引的合并策略；
- 对已安装文件的安全覆盖；
- 补丁应用失败时的回滚；
- 应用后正式目录的完整性复核。

因此**不要把 `--target preload` 理解成已经完成“预下载并应用更新”**。它不会自动应用补丁。

### P1：升级、修复和安装状态管理尚未完成

- 尚未实现完整的 upgrade / repair 工作流。
- 尚未保存和迁移原版 `_index/<version>` 兼容索引。
- 不会自动清理旧版本中已经废弃的文件。
- 不会检测已有游戏目录是否属于正确游戏、正确渠道或正确版本。
- 不会自动发现 `launchEntry`、游戏目录名或服务器返回的其他启动配置。
- 不会实现原版启动器的注册表、快捷方式和安装状态管理。

### P2：运行体验和防护尚有缺口

- 没有交互式暂停/继续；中断后可通过 `.part` 文件尝试续传。
- 尚无下载限速、磁盘空间预检查、可配置超时和完整的网络空闲超时。
- 单个文件失败后当前 worker 队列仍可能继续处理其他文件，可能造成额外流量。
- manifest 目前直接写文件，尚未实现原子替换和崩溃恢复。
- ZIP 解压尚无针对压缩炸弹的总解压大小、文件数量和压缩比限制。
- `verify` 检查 manifest 中的文件，不会报告所有额外文件，也不会清理孤儿文件。
- MD5/CRC32 是兼容原版资源清单的完整性检查，不是数字签名，也不能单独证明资源来源可信。
- 没有游戏启动功能，不配置 Proton，不处理反作弊、运行库或游戏运行问题。

## 4. 当前自动化测试覆盖范围

当前 Go 自动化测试覆盖以下行为：

1. 已有 `.part` 文件时，服务端返回合法 `206 Content-Range`，下载器能续传并校验。
2. 能解析一条逐文件索引记录，并拒绝 `../` 路径穿越。
3. 服务端忽略 Range 并返回 `200` 时，下载器退回完整下载。
4. 逐文件索引兼容首行计数，并拒绝异常计数或中途出现的三列记录。

它们不覆盖真实官方 API、原生模块、AES 参数提取、真实索引样本、CDN 轮换、断点续传中断恢复、切片合并、ZIP 解压、预下载、升级、磁盘空间不足和长时间网络异常。

GitHub Actions 已配置这些测试和构建步骤，但在取得成功的 workflow run 记录之前，不要宣称 CI 全部通过。

## 5. 验收标准

在将项目标为可用之前，至少需要完成：

- [x] Go 单元测试通过，amd64 Linux 构建成功。
- [x] 本机可从提供的原始 EXE 提取 AES 参数，并解密真实索引。
- [x] bootstrap 从官方 API 得到 `code=0` 且包含预期 `released` package 的响应。
- [x] 两种 `--probe` 均通过官方索引 MD5、解密和解析。
- [x] 两种 `--probe` 均下载真实资源并完成文件大小、MD5 校验。
- [x] 用预置 `.part` 确认官方 CDN 的 Range 续传及最终校验。
- [ ] 完成逐文件包的全量下载与 `verify`。
- [ ] 如果服务器实际选择切片包，另行完成切片下载、合并 ZIP、解压和 `verify`。
- [ ] 确认游戏目录结构、关键资源和实际启动入口与官方安装结果一致。
- [ ] 如果要支持预下载，必须另行完成差异合并和失败回滚测试。

