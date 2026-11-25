# 当前实现状态与未完成事项

> 更新日期：2026-11-25  
> 状态：开发版 / 两种包类型的小文件实服探测通过；自动安装、预下载、升级和修复代码已加入，完整流程仍待本地实服验收。

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
| 元数据获取 | 自动调用原版 N-API bootstrap；packageconfig 已实测，config/pre-check 为可选请求 |
| 下载模式选择 | 按原版 downloadMode=1/2 自动选择 packageType=1/2，支持手动覆盖 |
| 首次安装 | install 支持切片 ZIP 解压和逐文件安装 |
| 预下载 | 对比 released/preload 的 packageType=2 索引，仅下载 MD5 有变化的文件 |
| 升级 | 使用新旧索引补齐新文件、复用已验证预下载文件，并清理旧 manifest 追踪文件 |
| 修复 | 对当前版本全部资源重新计算 MD5，补齐缺失或损坏文件 |
| 版本状态 | 写入兼容原版的 _index/<version> 与 _index/version，并保存本地 manifest |

### 当前命令行为

- `install` 默认按原版 `downloadMode` 自动选择包：1=SLICE/packageType 1，2=REALITY/packageType 2；`--mode slice|reality` 可手动覆盖。
- `update` 与 `repair` 固定使用 packageType 2 逐文件索引，与原版更新路径一致。
- `preload` 同时获取 released 和 preload 的 packageType 2 索引，只暂存路径或 MD5 不同的资源。
- `update` 会优先复用通过新版本 MD5 校验的预下载文件，再执行完整文件校验；成功后才清理旧 manifest 中已废弃的文件。
- `info` 显示当前版本、downloadMode、包类型和可用的游戏/启动元数据。
- 新工作流未指定 `--packageconfig` 时，会自动运行 bootstrap；默认每次刷新匿名元数据。显式指定 packageconfig 时使用本地文件。
- `download --probe` 仍是小文件探测命令，不执行完整安装。
- `verify` 根据当前 manifest 校验逐文件资源；切片安装若无法取得 type-2 基线，则使用 ZIP 解压后的 CRC32/大小清单校验。
- `verify --target preload --version VERSION_CODE` 检查预下载暂存目录。

## 3. 尚未完成的部分

### P0：官方 API 客户端仍依赖原版 Windows N-API

- 当前没有纯 Go 官方 API 签名实现。Linux 工作流调用用户原始安装器中的 Electron/N-API 模块，由 Wine 执行一次性 bootstrap。
- packageconfig 匿名请求已在本机 Wine 10.0 / SC000 样本上实测通过；config 与 pre/check 是新增可选请求，尚未单独取得真实成功记录。
- 不支持账号登录、uid/utoken、token 刷新及需要登录的接口。
- bootstrap 固定使用 SC000 和简化 client-info；其他渠道、Windows 原生环境、不同 Wine 版本仍待验证。
- 自动工作流默认刷新 packageconfig；显式传入 --packageconfig 时可离线使用已有响应。

### P0：完整安装链路尚未完成实服验收

本机已完成两种包类型的小文件官方 CDN 探测、MD5/大小校验和 Range 续传，但以下环节仍需真实完整测试：

1. install 选择正确的 downloadMode 并完成整包安装；
2. SLICE 模式完成全部分片下载、ZIP 合并、解压和解压后 CRC32/大小校验；
3. REALITY 模式完成全部 40,457 个文件下载与最终 verify；
4. 安装目录结构、官方 `_index/version` 状态与实际启动入口一致；
5. 完整更新、修复和预下载流程在官方版本变化时跑通。

### P1：预下载、升级与修复已实现代码，尚未端到端验证

- preload 读取当前 released 和 preload 两份 packageType 2 索引，按相对路径与 MD5 比较，仅下载差异文件到 `_pre_download/<version>`。
- update 使用新版本索引重新验证暂存文件，复用有效预下载文件，下载其余文件；成功后清理旧 manifest 中已废弃的文件。
- repair 使用当前版本 packageType 2 索引逐项检查 MD5，只补齐损坏或缺失文件。
- 当前更新采用逐文件 .part + 校验 + rename，支持失败后重新运行继续；**没有整次升级的全局事务回滚**。如果更新中断，部分新文件可能已经落盘，但版本标记只在成功完成后更新，重新执行 update 可继续收敛。
- `gameExcludedFileList` 现在作为清理保护规则使用；原版样本中虽有 parseExcludeList 方法，但未找到其在升级路径的调用，因此 Linux 侧的保护行为属于更谨慎的实现，而不是声称逐字复刻原版。
- 不会清理旧 manifest 未追踪的未知文件；不会扫描并删除用户自建内容。

### P1：仍未实现的版本管理细节

- 没有自动从本机 Windows 注册表迁移既有安装路径或版本信息。
- 没有对其他渠道、不同 launcherId、不同安装器版本做兼容矩阵。
- 没有对服务端 downloadMode 之外的其他安装策略字段做全面建模。
- 没有实现原版所有异常恢复、安装状态事件和遥测流程。

### P2：运行体验与安全防护缺口

- 没有交互式暂停/继续；Ctrl+C 后可通过 .part 文件尝试续传。
- 尚无下载限速、磁盘空间预检查、可配置超时和完整的网络空闲超时。
- 单个文件失败后 worker 队列仍可能继续处理其他文件，可能造成额外流量。
- ZIP 解压尚无针对压缩炸弹的总解压大小、文件数量和压缩比限制。
- verify 检查 manifest 中的文件，不会报告所有额外文件，也不会清理孤儿文件。
- MD5/CRC32 是兼容原版资源清单的完整性检查，不是数字签名，也不能单独证明资源来源可信。
- 没有游戏启动功能，不配置 Proton，不处理反作弊、运行库或游戏运行问题。

## 4. 当前自动化测试覆盖范围

当前 Go 自动化测试覆盖以下行为：

1. HTTP Range 续传与服务端忽略 Range 时的完整重下。
2. 逐文件索引记录、三列计数头及路径穿越拒绝。
3. downloadMode 到 packageType 的自动映射和显式模式覆盖。
4. 排除文件/目录匹配与旧版本文件清理保护。
5. 预下载文件 MD5 验证后才进入正式目录。
6. ZIP 条目清单提取、解压后 CRC32/大小校验及损坏检测。

这些测试使用本地数据和测试目录，不代表完整游戏 CDN 安装、升级或启动已通过。

GitHub Actions 已配置这些测试和构建步骤，但在取得成功的 workflow run 记录之前，不要宣称 CI 全部通过。

## 5. 验收标准

在将项目标为可用之前，至少需要完成：

- [x] Go 单元测试通过，amd64 Linux 构建成功。
- [x] 本机可从提供的原始 EXE 提取 AES 参数，并解密真实索引。
- [x] bootstrap 从官方 API 得到 `code=0` 且包含预期 `released` package 的响应。
- [x] 两种 `--probe` 均通过官方索引 MD5、解密和解析。
- [x] 两种 `--probe` 均下载真实资源并完成文件大小、MD5 校验。
- [x] 用预置 `.part` 确认官方 CDN 的 Range 续传及最终校验。
- [ ] 完成 REALITY 模式的全量下载与 `verify`。
- [ ] 完成 SLICE 模式的全量分片下载、合并 ZIP、解压和 `verify`。
- [ ] 完成 preload 差异下载，并确认 update 复用预下载文件后仍通过最终 verify。
- [ ] 完成真实版本升级与 repair 测试，确认旧文件清理范围正确。
- [ ] 确认游戏目录结构、关键资源、`_index/version` 和实际启动入口与官方安装结果一致。
- [ ] 测试中断、磁盘空间不足和网络故障后的恢复行为。

