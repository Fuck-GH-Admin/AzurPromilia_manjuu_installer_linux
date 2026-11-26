# 当前实现状态与未完成事项

> 更新日期：2026-11-26
>
> 状态：开发版 / SC000 正式版 1.0.0.30 的 REALITY 全量安装、40,457 文件校验、单文件 repair 和 Proton 进入游戏已验证。SLICE 全量安装、预下载和跨版本升级待验收。

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
- SC000 样本在 Wine 10.0 下实测通过匿名 packageconfig、config 和 pre/check。config 返回游戏目录名，pre/check 返回当前版本的启动入口。
- 不支持账号登录、uid/utoken、token 刷新及需要登录的接口。
- bootstrap 固定使用 SC000 和简化 client-info；其他渠道、Windows 原生环境、不同 Wine 版本仍待验证。
- 自动工作流默认刷新 packageconfig；显式传入 --packageconfig 时可离线使用已有响应。

### P0：SLICE 完整安装待实服验收

本机 `install` 按官方 `downloadMode=2` 选择 REALITY，全量安装并检查了 40,457 个文件。安装版本、索引文件与官方清单一致，官方入口已落盘。[实服验收和空间数据](NEXT_LIVE_TEST.md#8-2026-11-26-manjuu--sc000-实测)

SLICE 已通过小文件探测和分片续传。完整分片下载、ZIP 合并、解压以及解压后 CRC32 / 大小校验仍待实测。

当前正式版的 SLICE 索引声明 752 个分片，合并 ZIP 为 75,145,275,761 bytes。分片和合并 ZIP 会在同一文件系统短时共存，因此在尚无有效 ZIP 缓存时，开始下载前至少需要预留“缺失分片字节数 + ZIP 大小”；全部分片均缺失时约为 150,290,551,522 bytes（150.29 GB / 140.0 GiB），尚不含解压后的游戏文件。新实现会在下载前合并检查分片下载与 ZIP 合并空间，避免磁盘不足时先下载几十 GB 再失败。解压所需空间只能在 ZIP 中央目录可读后计算，会在实际解压前再次检查。

### P1：预下载和跨版本升级待实测

- preload 读取当前 released 和 preload 两份 packageType 2 索引，按相对路径与 MD5 比较，仅下载差异文件到 `_pre_download/<version>`。
- update 使用新版本索引重新验证暂存文件，复用有效预下载文件，下载其余文件；成功后清理旧 manifest 中已废弃的文件。
- repair 使用当前版本 packageType 2 索引逐项检查 MD5。本机关闭代理后实测移走一个已验证的 4 字节资源，repair 只重新下载该文件，MD5 恢复。
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
- 下载限速、可配置超时和完整的网络空闲超时仍未实现。
- 下载工作流现在会在首个文件最终失败后停止派发新任务，并取消其他正在进行的 HTTP 请求；单个文件仍会先按现有策略重试。明确的 HTTP 4xx 永久错误（408、425、429 除外）会立即切换下一个 CDN。`verify` 不消耗网络，因此仍会扫描完整清单并汇总失败。
- 下载前会校验现有文件并估算缺失/损坏文件所需空间；完整且 MD5 验证通过的 `.part` 会作为可复用文件，不重复计算下载空间。SLICE 合并前检查 ZIP 大小，解压前检查缺失/损坏条目的解压空间。空间来自 `statfs` 的非特权可用块数；不构成空间预留，不能覆盖用户配额和其他进程并发占用。
- ZIP 合并前检查全部分片大小之和与官方声明 ZIP 大小一致，并限制单片复制长度；ZIP 解压校验限制条目数量，拒绝重复路径、文件/目录冲突、路径穿越、符号链接和特殊文件；累计解压大小时检查整数溢出，解压流限制为索引声明大小加 1 字节，并在写入前检查可用空间。没有使用固定压缩比阈值，以免误伤正常资源包。
- 下载根目录、游戏元数据目录、索引目录和预下载目录会拒绝已存在的符号链接重定向。
- verify 检查 manifest 中的文件，不会报告所有额外文件，也不会清理孤儿文件。
- MD5/CRC32 是兼容原版资源清单的完整性检查，不是数字签名，也不能单独证明资源来源可信。
- 下载器不负责启动游戏、安装 Proton 或修改 Steam。安装后的 Proton/Steam 配置仅提供手动步骤和启动选项示例；本机使用 `dwproton-11.0-16`、原生 Wayland 和独立 CompatData 前缀已进入游戏。[Proton 手动配置参考](PROTON_SETUP.md)

## 4. 当前自动化测试覆盖范围

当前 Go 自动化测试覆盖以下行为：

1. HTTP Range 续传与服务端忽略 Range 时的完整重下。
2. 逐文件索引记录、三列计数头及路径穿越拒绝。
3. downloadMode 到 packageType 的自动映射和显式模式覆盖。
4. 排除文件/目录匹配与旧版本文件清理保护。
5. 预下载文件 MD5 验证后才进入正式目录。
6. ZIP 条目清单提取、解压后 CRC32/大小校验及损坏检测。
7. 下载 worker 首错后停止派发、取消在途任务；verify 模式继续扫描所有文件；永久 HTTP 4xx 会快速切换 CDN。
8. 缺失/损坏文件的磁盘空间估算、非普通文件目标拒绝及整数溢出检查。
9. ZIP 文件/目录路径冲突、解压空间估算、已存在文件 CRC32 复用和分片总大小/合并归档大小一致性。
10. SLICE 下载前将缺失分片字节数与后续合并 ZIP 的保留空间合并预检，拒绝整数溢出；有效 ZIP 已缓存时跳过分片下载。

这些测试使用本地数据和测试目录，不代表完整游戏 CDN 安装、升级或启动已通过。

GitHub Actions 验证记录：

- 原完整工作流 Run #50（`45559651106`）通过 `go test -race ./...`、`go vet ./...`、Python/Node 检查、Linux amd64/arm64 构建及 Artifact 上传。
- 第一阶段可靠性改动 Run #111（`45619034089`，head `272ef2f`）通过上述全部检查。
- 随后加入分片总大小一致性、解压目标类型保护和更完整的空间/进度信息后，Run #139（`45620110972`，head `16019db`）再次通过 `go test -race ./...`、`go vet ./...`、Python/Node 检查、Linux amd64/arm64 构建及 Artifact 上传。
- 加入 SLICE 下载 + ZIP 合并的前置联合空间检查、有效 ZIP 复用和手动 Proton 配置文档后，Run #218（`45783753599`，head `8d3c2ed`）通过 `go test -race ./...`、`go vet ./...`、Python/Node 检查、Linux amd64/arm64 构建及 Artifact 上传。

## 5. 验收标准

在将项目标为可用之前，至少需要完成：

- [x] Go 单元测试通过，amd64 Linux 构建成功。
- [x] 本机可从提供的原始 EXE 提取 AES 参数，并解密真实索引。
- [x] bootstrap 从官方 API 得到 `code=0` 且包含预期 `released` package 的响应。
- [x] 两种 `--probe` 均通过官方索引 MD5、解密和解析。
- [x] 两种 `--probe` 均下载真实资源并完成文件大小、MD5 校验。
- [x] 用预置 `.part` 确认官方 CDN 的 Range 续传及最终校验。
- [x] 完成 REALITY 模式的全量下载与 `verify`，40,457 个文件全部通过。
- [ ] 完成 SLICE 模式的全量分片下载、合并 ZIP、解压和 `verify`。
- [ ] 完成 preload 差异下载，并确认 update 复用预下载文件后仍通过最终 verify。
- [x] 完成同版本 repair 实测，只补齐缺失资源。
- [ ] 完成真实跨版本升级，确认旧文件清理范围正确。
- [x] 确认 REALITY 目录与官方资源清单一致，`_index/version` 与本地 manifest 版本一致，官方入口文件已落盘。
- [x] 在 Linux 启动 SC000 游戏客户端，到达登录流程并完成游戏内资源更新。
- [x] 用户确认已通过手动 Proton 配置进入并运行游戏；长期运行稳定性未作专项压力测试。
- [ ] 测试中断、磁盘空间不足和网络故障后的恢复行为。
- [x] worker 取消、磁盘预检查与 ZIP 回归测试通过 GitHub Actions。



## 6. GitHub Actions 真实公网探测（2026-11-26 凌晨）

为确认是否可以在用户本机断电期间先用 GitHub hosted runner 做真实服务器探测，临时加入并运行了 Windows Actions 公网探针。结果如下：

- Run #3（`45660232078`）从 GitHub Windows hosted runner 直接访问 `https://azurpromilia.jp/` 和 `/PCdownload`，均得到 HTTP 200。
- 同一 runner 直连 `https://api-grp.manjuu.com/` 和 `https://api-grp-l2.manjuu.com/`，根路径均得到 HTTP 404。这证明 DNS、TLS 和 HTTP 出站链路正常；404 只是根路径没有业务路由。
- `PCdownload` 页面能解析出 Yostar 官方安装器地址，host 为 `launcher-pkg-azupro-jp.yo-star.com`。Actions 实际下载了 179,893,048 字节，SHA-256 为 `dde29b4fa4a6eabfb664838c807460e65746f590f1a51754a33d976763bb1e3f`。
- 该 SHA 与本项目已逆向的 Manjuu/SC000 样本 `cf4b1f6233af2f2cb2d1c0de12f54d07606c3c84c829390b6caccfb2d0d1f04b` 不同，因此第一轮按预期停止，没有把未知新包误当作旧样本。
- Run #5（`45660583827`）在确认下载来源和新 SHA 后继续测试。外网检查和安装器下载均成功，但 `tools/bootstrap.py` 对该日本 Yostar 安装器报 `embedded launcher ZIP not found`。因此它不是当前 Manjuu/SC000 bootstrap 所支持的同一安装器封装格式。
- 结论：**GitHub Actions 可以访问官方公网和下载大型官方安装器；当前阻塞不是 Actions 网络，而是测试安装器渠道/封装格式不一致。**

这两次 Actions 没有完成 Manjuu/SC000 的真实 packageconfig + CDN 小资源验证，因此不能替代后续使用正确 SC000 安装器进行的实服测试。临时自动探针在取证后删除，避免每次 PR 更新都重复下载约 171.5 MiB 的日本安装器；原有手动 `.github/workflows/live-probe.yml` 保留。
