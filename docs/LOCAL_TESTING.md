# 本地测试与问题反馈

> 本文用于用户在自己的 Linux 机器上验收。优先使用独立测试目录，不要第一次就对已有正式游戏目录执行下载或覆盖。

## 1. 准备环境

- Linux x86-64（首轮建议）；
- Go 1.22 或更新版本；
- Python 3；
- 原始安装器 `AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe`；
- 若在 Linux 上运行一次性 metadata bootstrap，需要可用的 64 位 Wine 环境；也可以在 Windows x64 上运行 bootstrap；
- 足够的磁盘空间和稳定网络。

下载器本体不需要 Wine、Proton、Electron 或 Python 第三方包。Wine 只用于尝试调用原安装器附带的 Windows Electron/N-API 模块取得官方包清单。

## 2. 拉取和编译

```bash
git clone https://github.com/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux.git
cd AzurPromilia_manjuu_installer_linux
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

## 3. 获取官方包信息

在 Linux + Wine 环境：

```bash
python3 tools/bootstrap.py \\
  --installer "$HOME/Downloads/AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe" \\
  --out ./packageconfig.json
```

如果 Wine 可执行文件不叫 `wine`，可传入 `--wine /path/to/wine`。在 Windows x64 环境可直接运行同一 Python 命令，不需要 `--wine`。

成功时应生成本地 `packageconfig.json`。该文件可能包含临时 CDN URL，不要提交到 Git、不要上传到公开 issue，也不要把账号 token、Cookie、签名头或密钥贴到聊天中。

如果失败，记录：
- Python 的完整错误；
- Wine 版本和架构（如使用 Wine）；
- 是否能启动原始安装器；
- 原生模块加载错误或官方 API 返回 code；
- 不要公开 `app_config.dat` 或 `secretKey`。

注意：bootstrap 当前为实验性匿名请求。它不支持账号登录，也没有完整复现原版设备标识、trace ID 和 `grp-client-info`，失败并不代表 CDN 下载引擎本身失败。

## 4. 第一轮：只做真实服务器小文件探测

建议使用全新的空目录：

```bash
mkdir -p "$HOME/Games/AzurPromilia-probe"
./azurpromilia download \\
  --installer "$HOME/Downloads/AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe" \\
  --packageconfig ./packageconfig.json \\
  --root "$HOME/Games/AzurPromilia-probe" \\
  --workers 2 \\
  --probe
```

预期行为：

1. 从安装器读取 AES 参数；
2. 下载并校验官方索引文件；
3. 解密、解析索引；
4. 选择索引中最小的非空资源（不超过 64 MiB）；
5. 从 CDN 下载该资源并校验大小和 MD5；
6. 输出 `LIVE PROBE OK`。

`--probe` 不会安装游戏文件，只会在测试目录 `.azurpromilia/` 下留下索引和探测资源。当前默认选择 `packageType=2`。若官方响应只有 `packageType=1`，需添加 `--slice`；不要假设两种模式都可用。

如果失败，请记录从 `VERSION`、`RETRY`、`FAIL` 到最终 `ERROR` 的完整日志，以及 HTTP 状态码和响应错误。不要只截取最后一行。

## 5. 第二轮：逐文件包完整下载

只有 `--probe` 成功后才建议开始：

```bash
./azurpromilia download \\
  --installer "$HOME/Downloads/AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe" \\
  --packageconfig ./packageconfig.json \\
  --root "$HOME/Games/AzurPromilia-test" \\
  --workers 4
```

下载完成后：

```bash
./azurpromilia verify --root "$HOME/Games/AzurPromilia-test"
```

目前默认下载 `packageType=2`。CLI 还没有自动读取 `downloadMode` 来选择包类型；如果官方响应指示使用切片模式，请不要把默认逐文件模式的失败当成 CDN 故障。

如需检查断点续传，可在下载一个较大的文件时按 Ctrl+C，然后原命令重新运行。观察是否发送 Range 请求并正确续传。若服务器忽略 Range 并返回 200，程序应从头覆盖 `.part`，不能把完整响应追加到旧分片。

## 6. 第三轮：切片包（仅在服务器确实提供时）

```bash
./azurpromilia download \\
  --installer "$HOME/Downloads/AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe" \\
  --packageconfig ./packageconfig.json \\
  --root "$HOME/Games/AzurPromilia-slice-test" \\
  --workers 4 \\
  --slice
```

默认会合并 ZIP 并解压。随后：

```bash
./azurpromilia verify --root "$HOME/Games/AzurPromilia-slice-test"
```

如果只想测试切片下载和 ZIP 合并、不解压，可使用 `--no-extract`；此时 `verify --no-extract` 只验证切片和合并后的 ZIP，不检查解压目录。

## 7. 预下载模式：仅验证暂存，不应用

```bash
./azurpromilia download \\
  --installer "$HOME/Downloads/AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe" \\
  --packageconfig ./packageconfig.json \\
  --root "$HOME/Games/AzurPromilia-test" \\
  --target preload
```

查看对应版本号后：

```bash
./azurpromilia verify \\
  --root "$HOME/Games/AzurPromilia-test" \\
  --target preload \\
  --version VERSION_CODE
```

**不要手工把 `_pre_download` 目录覆盖进正式游戏目录。** 当前版本尚未实现原版的预下载差异比较、合并、回滚和正式更新流程；这里只能验证暂存下载是否正确。

## 8. 反馈时请提供什么

请按以下顺序反馈，便于定位：

1. Linux 发行版、架构、Go 版本、Wine 版本（如使用）；
2. 运行的完整命令（删除本机用户名等隐私路径即可）；
3. 完整错误日志；
4. 失败阶段：bootstrap、索引下载、索引解密、索引解析、资源下载、MD5 校验、ZIP 合并、ZIP 解压或最终 verify；
5. 如需分享 `packageconfig.json`，先删除 UID、token、Cookie、签名等字段，并把 CDN URL 的路径和查询参数删掉，只保留必要的字段名、packageType、版本号、文件大小和域名。通常只需要贴 `code`、`versionCode`、`versionName`、`downloadMode`、`packageType`、`fileSize`、`fileMd5` 和 CDN 主机名。

不要上传原始 `packageconfig.json`、`app_config.dat`、`config.json`、`secretKey`、账号 token、Cookie 或任何个人设备标识。

## 9. Proton

下载器不启动游戏，也不修改 Proton。待资源下载和校验完成后，再将实际游戏入口添加到 Steam，选择 Proton/GE-Proton，并使用独立 CompatData 前缀。游戏入口和实际运行兼容性以完整安装后检查到的文件为准。
