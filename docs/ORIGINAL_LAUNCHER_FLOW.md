# 原版启动器安装、更新与预下载流程逆向记录

> 样本：`AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe`
>
> 样本 SHA-256：`cf4b1f6233af2f2cb2d1c0de12f54d07606c3c84c829390b6caccfb2d0d1f04b`
>
> 分析对象：安装器内嵌 Electron ASAR 的 `main.js`。本文件记录的是样本中实际可见的客户端逻辑，不代表服务端所有隐藏规则均已知。

## 1. API 入口

原版客户端的 API 封装可确认以下 GET：

| 用途 | 路径 | 查询参数 |
|---|---|---|
| 游戏基础信息 | `/pc-launcher/client/gameversion/config` | `launcherId` |
| 游戏包清单 | `/pc-launcher/client/gameversion/packageconfig` | `launcherId`, `uid` |
| 当前版本启动信息 | `/pc-launcher/client/gameversion/pre/check` | `launcherId`, `gameVersionCode`, `uid` |

原版 GET 签名由 `cutils.extApi3(timestamp + querystr(data) + utoken)` 生成。匿名请求的 `uid`、`utoken` 为空。查询参数由原版 `querystr` 生成；请求头由启动器配置、平台信息和 `delyinfo.json` 提供。

当前 Linux 工程的 bootstrap 继续使用原版 N-API 模块签名，不伪造签名或绕过授权。它会请求 packageconfig，并尝试补充 config 与 pre/check 的匿名响应。pre/check 若要求登录或返回错误，会被记录为可选元数据不可用，不阻断下载。

## 2. 下载模式与包类型

原版枚举：

| `downloadMode` | 原版枚举 | 对应包 |
|---:|---|---|
| 1 | `SLICE` | `packageType=1`，分片 ZIP |
| 2 | `REALITY` | `packageType=2`，逐文件资源 |

原版 `downloadMode` 缺省时回退为 `SLICE`。服务器 packageconfig 可以同时返回两种 package，但安装时根据 `downloadMode` 选择。

## 3. 首次安装

### 3.1 REALITY / packageType 2

1. 下载 packageType 2 的加密索引。
2. 以索引返回的 MD5、文件大小验证密文。
3. Base64 解码并使用 AES-CBC 解密。
4. 解析索引：4 列记录是目录，5 列记录是文件。
5. 逐文件检查目标文件大小和 MD5；不匹配或缺失的文件下载到目标目录。
6. 下载完成后写入 `_index/<version>`，并保存 `_index/version`（包含 `version`、`versionName`）。

### 3.2 SLICE / packageType 1

1. 下载 packageType 1 的加密索引。
2. 解密索引。3 列记录描述最终 `game.zip` 的大小和 MD5；5 列记录描述分片。
3. 下载所有缺失或校验失败的分片。
4. 按索引顺序合并为 `game.zip`。
5. 对合并后的 ZIP 执行大小和 MD5 校验。
6. 解压到游戏目录。
7. 原版删除临时分片和 ZIP。
8. 如果 packageconfig 同时提供 packageType 2，原版还会异步下载其索引到 `_index/<version>`，作为后续版本管理基线。

## 4. 升级与修复

原版 `upgradeGame` 使用 packageType 2 的逐文件索引，即使首次安装采用 SLICE 模式，升级路径也会使用 REALITY 索引。

升级：

1. 下载新版本 packageType 2 索引，解密并解析新文件列表与目录列表。
2. 读取旧版本 `_index/<lastVersion>`，解析旧文件列表与目录列表。
3. 检查新版本所需文件；已存在且校验通过的文件跳过，缺失或不匹配的文件重新下载。
4. 全部下载成功后，删除旧索引中存在、但新索引中已经不存在的文件。
5. 对旧目录执行空目录删除；非空目录保留。
6. 删除旧版本索引文件，写入新版本索引和 `_index/version`。

修复：

- 使用当前 released packageType 2 索引。
- 对全部文件重新检查大小和 MD5。
- 只重新下载缺失或损坏文件。
- 不执行旧版本文件清理。

Linux 实现采用更严格的策略：每个新文件都实际计算 MD5，不沿用原版升级检查中“同路径且旧索引 MD5 相同即可跳过重新计算”的优化假设。

## 5. 预下载

原版预下载 agent 的实际步骤：

1. 选择 preload 的 packageType 2 包。
2. 下载 preload 索引到 `_index/<preloadVersion>`。
3. 下载当前 released packageType 2 索引到 `_index/<currentVersion>`。
4. 解密两个索引，构建当前文件路径到 MD5 的映射。
5. 对 preload 文件逐项比较：如果当前版本存在同路径且 MD5 相同，则跳过；其余文件进入预下载队列。
6. 将变化文件下载到 `_pre_download/<preloadVersion>/<relativePath>`。
7. 预下载完成后记录完成版本。

原版升级时，如果 `_pre_download/<newVersion>` 存在，会把其中的文件移动到游戏目录，然后继续执行新版本完整索引检查与缺失文件下载。预下载只是提前搬运下载结果，不代替最终版本索引校验。

当前 Linux 实现对预下载文件先按新版本索引重新验证 MD5，再以同文件系统原子 rename 放入目标路径；随后仍执行完整更新校验。预下载目录仅在更新成功后清理，避免原版“合并出错仍清空目录”的风险。

## 6. 排除列表与清理

原版 `gameExcludedFileList` 项包含 `filePath`、`fileType`；原版 `parseExcludeList` 中 `fileType=1` 表示目录，其他类型按单文件处理。原版清理还默认保护 `_index`、`_pre_download`。

在本样本的 `main.js` 中能找到 `parseExcludeList` 方法和 `gameExcludedFileList` 参数传递，但未找到该方法在当前升级路径中的实际调用。因此不能声称原版升级清理一定执行了全部服务端排除规则。

Linux 实现将 `gameExcludedFileList` 用作清理保护规则，并额外始终保护 `_index` 与 `_pre_download`。清理范围只限旧 manifest 明确追踪的文件，不扫描并删除用户目录里的任意未知文件。

## 7. 版本和启动元数据

原版基础信息响应中的 released 项用于读取 `gameName`、`gameFolderName`、`versionCode`、`versionName`、`launchEntry`、`packageSize` 等字段。

`/pre/check` 的响应会提供 `launchEntry`、`launchParams`、`gameRuntimeParams`。原版把它们用于确定游戏 EXE、命令行参数和运行时路径配置。当前 Linux 工程只保留并显示这些元数据，不负责启动游戏，不设置 Proton，不模拟 Windows 注册表或游戏账号环境。

## 8. Linux 命令与原版流程映射

| Linux 命令 | 对应流程 |
|---|---|
| `info` | 获取并显示当前 released、downloadMode、包类型、preload 和可用启动元数据 |
| `install` | 首次安装；默认按服务端 downloadMode 自动选择 SLICE / REALITY |
| `repair` | 用当前 packageType 2 索引校验并补齐当前版本文件 |
| `preload` | 对比 released/preload 两份 packageType 2 索引，只下载变化文件 |
| `update` | 获取新版本 packageType 2 索引、复用已验证预下载文件、补齐新文件、清理旧 manifest 文件 |
| `verify` | 按本地 manifest 独立检查已安装文件 |

## 9. 尚未由当前样本证明的事项

- 登录、token 刷新及需要账号态的接口行为。
- 其他子渠道、其他 launcherId 或不同版本安装器的响应差异。
- 服务端对排除列表的完整语义及更多 `fileType` 枚举。
- 全量游戏安装、完整升级和真实游戏启动结果。
- 游戏启动后是否还需额外运行库、反作弊或平台组件。

这些项目不应通过模拟服务器响应来宣称完成；需要用户本地真实环境或官方服务端证据。
