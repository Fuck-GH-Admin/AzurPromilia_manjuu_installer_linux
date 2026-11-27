# Yostar JP download implementation — 2026-11-27

## Verified source and static evidence

The exact official sample downloaded by Actions run 45868787331 was copied into the research container from artifact 15140919290. Local SHA-256 independently matched `dde29b4fa4a6eabfb664838c807460e65746f590f1a51754a33d976763bb1e3f`, size 179893048 bytes. The PE32 NSIS outer archive uses non-solid raw deflate blocks; the app-32.7z payload (SHA-256 8435b0019c80a46436952a22a6f0425218681704d07e507aaade5dbba60bf219) was independently extracted via libarchive. Inner resources/app.asar SHA-256: 4f013026736f6afa819a37f299caa9577140e88254ef4097b4d4589fa47f0818. Inner package.json identifies AZUPRO_JP_Gamelauncher 1.8.1. No installer execution was necessary.

Evidence pointers within the inner ASAR:
- out/main/index.js:863–944: anonymous HTTP client metadata header, manifest URL resolver, CDN resolver.
- node_modules/@launcher/utils/dist/index.cjs:2861–2882: local object hash and anonymous metadata signature.
- out/renderer/assets/main-4217f8b0.js:26–29 and 8080–8113: version, basis, executable name from game config.
- out/main/index.js:3917 onward: reflected CRC64 polynomial 0xc96c5795d7870f42; little-endian CRC complements the state. 19440–19531: decimal unsigned 64-bit hash strings. Standard CRC64/XZ check vector 123456789 = 11051210869376104954.
- out/main/index.js:19593–19648: manifest source and file array; comparison and repair.
- out/main/index-8b10f6cc.js:18675–18770: source + relative file path on selected CDN origin; HTTP Range and CRC64 verification.
- out/main/index-8b10f6cc.js:18902–18917: original local launcher manifest/config metadata.

## CLI draft

Build with `go build -o azurpromilia ./cmd/azurpromilia`. The Japanese channel is explicitly separate from SC000:

    ./azurpromilia yostar info
    ./azurpromilia yostar probe --root /tmp/azupro-jp-probe
    ./azurpromilia yostar install --root "$HOME/Games/AzurPromilia-JP" --workers 4
    ./azurpromilia yostar verify --root "$HOME/Games/AzurPromilia-JP"
    ./azurpromilia yostar repair --root "$HOME/Games/AzurPromilia-JP" --workers 4

Go fetches the released config, primary/backup CDN, and JSON manifest using the anonymous desktop-client metadata protocol. It converts decimal uint64 hashes to explicit crc64: checksums in the shared Entry representation. The legacy MD5 field is retained for SC000 compatibility; SC000 index parsing remains strictly MD5. The existing parallel download, partial resume, size limit, disk space preflight, cancellation and atomic file rename paths are reused. JP metadata is stored separately under .azurpromilia-yostar; SC000 state is refused at the JP root.

Install/repair converge to the latest manifest and do not remove unknown or obsolete files. Original Windows launcher metadata files are not yet emitted. No account access, proprietary executable runtime, GUI, Proton configuration, or auto-start. No claim of cross-version update compatibility or full install until live-tested.

## Verification

Local original-source analysis and unit test vectors are not official server evidence. The live Actions workflow must actually discover the released configuration, parse the manifest, download a nonempty official resource, and verify its CRC64 before marking the public protocol accepted. Full installation remains a separate acceptance test. Keep the PR as draft until verified; do not merge into main solely on static analysis.

## Real server acceptance (2026-11-27)

- Run 45962589151: go race tests, vet and build passed; real API/manifest fetched, then leading slash in official file path was rejected. Fixed by stripping exactly one leading slash before path validation.
- Run 45962747571: official released config and all 40,299 manifest entries parsed; shared disk preflight incorrectly retained an MD5-only constraint. Fixed in storage.go and extended regression test to go through runFiles.
- Run [45962884069](https://github.com/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux/actions/runs/45962884069), head df78ffe0ec3d861203b54233a3e29abcdb87ee71: race tests, vet, Linux build, real official metadata and CDN probe all passed. Official release 1.0.2; 40,299 files; 84,158,601,695 bytes; two CDN origins; launch name AzurPromiliaJP. Real resource `AzurPromiliaJP_Data/StreamingAssets/.res/raw_streaming_package/06ujhj4pltkkp3tyowbnng.r`, size 4 bytes, unsigned CRC64 17628643879108190027. Result: 1/1 verified, 0 failed, 4 bytes transferred. This is evidence of a working live metadata-to-file path, not full-install acceptance. No account or Wine used.

See the local test below for larger-resource throughput and interruption. Full installation, complete local verification, live repair after damage, and cross-version update and cleanup still require acceptance tests. The CRC64 resume test with an asserted Range offset uses a local HTTP server.

## Integration into current manager

Use `azp yostar -p "$HOME/Games/AzurPromilia-JP" status`, then `azp yostar install`, `verify`, `sync`. The JP directory is stored independently in CLI configuration; the main menu exposes an experimental JP entry. No complete JP installation or launch is claimed. The original research-only scripts and sample-acquisition workflow are recorded in the [closed research PR](https://github.com/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux/pull/6).

## 2026-11-27 本机直连与命令行复测

从 `main` 的 `b88c6f2` 在 Linux amd64 上重新编译，使用空游戏目录及独立配置。日服 `1.0.2` 的官方清单含 40,299 个文件，共 84,158,601,695 字节（78.38 GiB）。清除命令环境中的代理变量后，状态查询和 4 字节探针均成功。主、备 CDN 都对同一个 5,617,152 字节资源的前 256 KiB 返回 HTTP 206，响应范围和字节数符合请求。

用 `azp yostar -p "$game_dir" -v install` 直连下载，再用 `azp yostar -p "$game_dir" -j 32 -v install` 在同一目录继续。两次累计传输 **1,970,448,081 字节**。主动停止时有 357 个文件通过 CRC64，零失败、零重试，目录留有 29 个 `.part` 文件，尚未生成完整安装清单。4 并发实测约 0.3–0.6 MiB/s，32 并发在测试后段约 8–10 MiB/s，两个数字只反映本次测试的网络环境。本轮未逐文件确认续传的 Range 偏移，也未完成日服安装或启动。测试文件随后已清理。

本机下载日服无须代理。程序继续遵循标准代理环境变量。以下命令仅对这次探针使用直连，并将路径记录在临时配置里。

```bash
test_dir=$(mktemp -d)
env -u HTTP_PROXY -u HTTPS_PROXY -u ALL_PROXY \
    -u http_proxy -u https_proxy -u all_proxy \
    XDG_CONFIG_HOME="$test_dir/config" \
    ./azp yostar -p "$test_dir/game" probe
```

如需复测 `install`，请将游戏目录放在剩余空间足以容纳整份游戏清单的文件系统。即使打算提前停止，安装命令也会按全量清单检查可用空间。小文件 `probe` 可在空间较小的文件系统中运行。

### 已修复与验证

| 现象 | 修复与复测 |
| --- | --- |
| 主菜单空闲约 100 ms 就因 `EOF` 退出；`--no-color` 仍显示颜色 | 普通按键阻塞等待，只有收到 Esc 后才限时读取后续字节。在伪终端分别让主菜单和日服子菜单空闲超过 0.8 秒，验证分段到达的方向键、连续按键、Enter、Esc、`q`、无颜色输出及退出后恢复终端设置。清屏和光标控制仍保留。 |
| `yostar -p` 在未知选项、无效版本参数或多余位置参数报错前就写入路径 | 先完成参数解析与校验，再记住显式路径。有效路径仍在任务开始时保存，方便下载中断或网络失败后继续。回归测试覆盖无效参数下的配置保留。 |
| Ctrl+C 后显示 `下载 complete` | 汇总区分成功、失败与中断，同时保留退出状态和已下载的文件。修复版日服安装直连传输 5,026,956 字节、校验通过 1 个文件后发送 SIGINT，正常退出并报告 `interrupted`，没有生成完整安装清单。 |
| 未安装时 `yostar verify` 显示底层文件不存在错误并创建空游戏目录 | 缺少清单时提示执行 `yostar install`，保持游戏目录的原有状态。 |

`go test -race -count=1 ./...`、`go vet ./...` 以及 Linux amd64 和 arm64 构建均通过。arm64 在本轮只验证构建。

### 待确认的行为与方案

- **国服旧命令也会提前写入 `-p`。** 在独立配置下，`azp -p /tmp/cn-new status --bad-test-flag` 报错，但后续 `azp path` 显示 `/tmp/cn-new`。建议在国服工作流和旧 `download`、`verify` 的解析入口完成参数校验之后再持久化路径，保留 `azp path` 的显式设置行为和有效命令在网络失败后可直接重试的语义。先确认脚本是否依赖旧入口对多余位置参数的处理，再决定是否一并收紧。
- **工具更新检查受代理环境影响。** 同一台机器上，`azp upgrade --check` 继承代理时遇到 HTTP 403，清除代理变量后成功。建议在有代理且返回 403 时提示检查代理或允许用户显式选择直连。自动改用直连会改变用户指定的网络路径，需要先确认产品行为。
