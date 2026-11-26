# SC000 实服验收步骤与记录

目标：先把当前 **Manjuu / SC000** 下载器真实跑通。不要顺手测试日本 Yostar 包，不要混渠道结论。

## 1. 测试前

- 使用已经逆向过的原始安装器：
  `AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe`
- SHA-256：
  `cf4b1f6233af2f2cb2d1c0de12f54d07606c3c84c829390b6caccfb2d0d1f04b`
- 使用独立、可丢弃的空目录，例如：
  `$HOME/Games/AzurPromilia-test`
- 确认磁盘空间充足。新版本会在真正下载前打印：
  - 预计还需要的空间；
  - 当前文件系统可用空间。
- 不需要提供账号、Cookie、token 或设备标识。

## 2. 第一步：只看当前服务器状态

```bash
./azurpromilia info \
  --installer /path/to/AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe \
  --root "$HOME/Games/AzurPromilia-test"
```

需要记录：

- released versionCode / versionName；
- downloadMode；
- packageType 1 / 2 是否都存在；
- preload 是否存在、版本是否不同于 released；
- gameFolderName；
- launchEntry；
- config / pre-check 是否成功；
- 若失败，保留 HTTP/API 错误，不猜原因。

把输出保存下来即可，注意不要公开完整临时 CDN URL。

## 3. 第二步：真实首次安装

优先使用自动模式，不手工强制包类型：

```bash
./azurpromilia install \
  --installer /path/to/AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe \
  --root "$HOME/Games/AzurPromilia-test" \
  --workers 4
```

重点看：

- 实际选择的 packageType 是否与 downloadMode 一致；
- 磁盘空间预检查是否合理；
- 下载过程中是否出现 CDN 切换；
- 若某个文件最终失败，后续新任务是否停止派发；
- Ctrl+C 中断后再次执行能否利用 `.part` 续传；
- SLICE 模式若被选中，分片总大小是否与最终 ZIP 声明一致，合并和解压是否通过。

不要为了测试异常而中断第一次完整安装。先争取拿到一次干净的成功样本。

## 4. 第三步：全量校验

安装成功后：

```bash
./azurpromilia verify \
  --root "$HOME/Games/AzurPromilia-test"
```

要求：

- 最终失败数为 0；
- REALITY：全部 manifest 文件大小 + MD5 通过；
- SLICE：解压后的大小 + CRC32 通过；
- `_index/version`、本地 manifest 与安装版本一致。

如果 verify 有失败，不要马上重装。先保留：

- 失败文件相对路径；
- 预期大小 / MD5 或 CRC32；
- 实际文件大小；
- 最后使用的 CDN / HTTP 错误；
- 是否存在同名 `.part`。

## 5. 第四步：Repair

只在这个可丢弃测试目录中做。

任选一个 **manifest 明确追踪、体积不太大的普通资源文件**：

1. 先记录它的 MD5；
2. 删除或改坏该文件；
3. 执行：

```bash
./azurpromilia repair \
  --installer /path/to/AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe \
  --root "$HOME/Games/AzurPromilia-test" \
  --workers 4
```

4. 再运行一次 `verify`。

通过条件：

- 只重新下载损坏/缺失资源；
- 修复后的 MD5 恢复；
- 其他已校验文件不被重复下载；
- 最终 verify 0 失败。

## 6. Preload / Update 何时测

只有 `info` 明确显示真实 preload 或新的 released 版本时才继续。

若存在 preload：

```bash
./azurpromilia preload \
  --installer /path/to/original.exe \
  --root "$HOME/Games/AzurPromilia-test" \
  --workers 4
```

记录：

- preload version；
- 差异文件数量；
- 理论差异字节；
- 实际下载字节；
- 已缓存/跳过数量。

等该 preload 真正变成 released 后再执行：

```bash
./azurpromilia update \
  --installer /path/to/original.exe \
  --root "$HOME/Games/AzurPromilia-test" \
  --workers 4

./azurpromilia verify \
  --root "$HOME/Games/AzurPromilia-test"
```

若服务器没有真实版本变化，就先不测 update。不要手工改 versionCode 来伪造“升级成功”。

## 7. 最需要回传的东西

如果出现问题，优先回传：

- 完整命令；
- 从命令开始到错误结束的终端日志；
- `info` 的版本 / downloadMode / preload 摘要；
- 失败文件相对路径；
- 是否重跑、是否有 `.part`；
- 文件系统剩余空间；
- 程序退出码。

不要上传完整 `packageconfig.json`、临时 CDN URL、签名、secretKey 或任何账号凭据。

## 8. 2026-11-26 Manjuu / SC000 实测

Ubuntu 26.04.1 x86-64、Go 1.27.0、Wine 10.0，使用校验过 SHA-256 的原版 SC000 安装器。在独立的 ext4 游戏目录执行，官方 API 和 CDN 请求均走直连。获取测试分支时，GitHub 直连超时，Git 操作临时使用已有代理。

| 项目 | 实测结果 |
|---|---|
| 官方接口 | 包清单、`config`、`pre/check` 均成功 |
| `released` / `preload` | `1.0.0.30`（`1.0.0`），当次响应无预下载版本 |
| 下载模式 | `downloadMode=2`（REALITY），`install` 自动选择 `packageType=2` |
| 两种索引 | 逐文件索引 15,024,044 字节，切片索引 148,184 字节 |
| 游戏目录与入口 | `config` 提供 `AzurPromilia_Game`，`pre/check` 提供 `AzurPromilia.exe` |

逐文件索引声明 40,457 个文件、29 个目录，资源共 84,840,079,623 字节。下载前预检为 79.01 GiB，文件系统可用 94.98 GiB。REALITY 会把校验通过的文件直接放到游戏目录，适合本次约 100 GB 的空间预算。

切片索引声明 752 个分片，合并 ZIP 为 75,145,275,761 字节，分片大小之和与 ZIP 声明一致。SLICE 在同一文件系统同时保存分片和合并 ZIP，空目录安装时仅下载与合并阶段就至少需要 150,290,551,522 bytes（150.29 GB / 140.0 GiB），尚不含解压空间。程序现已在开始下载前把缺失分片字节数与 ZIP 大小合并预检；普通安装路径已有有效 ZIP 时可跳过分片下载。底层 `download --no-extract` 仍保留分片文件，以支持后续 `verify --no-extract`。解压空间会在读取 ZIP 中央目录后另行预检。服务端同时提供逐文件包时，可用 `install --mode reality` 在有限空间安装。本次服务端已选择 REALITY。

调查期间修复了 `info` 的目录和入口摘要，以及 `download --mode slice` 选中切片包却使用逐文件解析器的问题。修复后，自动逐文件模式、`--mode slice` 和兼容标志 `--slice` 均通过实服探测。

### 全量安装与校验

- 从 `3d70040` 构建的 `install` 使用 4 个 worker，在空测试目录完成 REALITY 安装，耗时约 32 分 15 秒。日志记录 `40,457/40,457` 个文件校验通过，下载 84,840,079,623 字节，退出码 0，零失败、零取消，也未记录重试或 CDN 切换。
- `_index/version`、本地版本文件和 manifest 的版本一致。`_index/1.0.0.30` 与通过校验的官方索引字节一致。官方 `pre/check` 返回的 `AzurPromilia.exe` 存在于安装根目录和 manifest 中，为 Windows x86-64 PE 文件。
- 安装目录占用 84,987,486,208 字节（约 84.99 GB，含索引和状态文件）。清理测试时创建的独立 Wine 前缀后，安装完成时文件系统可用约 14.19 GB。
- 用 `50f80ca` 构建的 CLI 单独执行 `verify`，40,457 / 40,457 个文件通过大小与 MD5 校验，耗时约 38 秒，退出码 0，无网络传输。
- 从测试目录移走一个 manifest 中的 4 字节资源后，以同一版本清单执行 `repair`。预检仅需 4 字节，官方 CDN 只传输了该资源。原 MD5 恢复，repair 退出码 0，耗时约 2 分 11 秒（含扫描现有文件）。修复后再次运行 `verify`，40,457 / 40,457 个文件通过，零失败。
- 复查官方包清单仍是 `1.0.0.30`，未提供预下载版本。完整 SLICE 安装和真实版本升级仍待实测。
- 游戏运行已另行完成手动 Proton 配置；用户反馈已成功进入游戏。下载器不负责启动或管理 Proton，配置说明见 [PROTON_SETUP.md](PROTON_SETUP.md)。


---

# Yostar 日本下载器：未来兼容计划（暂缓）

Yostar 日本启动器已确认与当前 Manjuu / SC000 安装器的外层封装不同，初步静态信息保存在 [YOSTAR_COMPAT_RESEARCH.md](YOSTAR_COMPAT_RESEARCH.md)。

**当前不开展 Yostar 逆向、下载器适配或 CI 自动探测。** 优先完成 Manjuu / SC000 的 SLICE 全量安装、预下载和跨版本升级实测。待主线稳定后，再评估是否通过独立 metadata/bootstrap adapter 扩展 Yostar 支持；不复制维护整套下载核心。
