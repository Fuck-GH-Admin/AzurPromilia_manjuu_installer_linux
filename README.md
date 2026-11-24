# AzurPromilia Manjuu Launcher — Linux CLI

原生 Linux 命令行下载器，目标是兼容《蓝色星原：旅谣》Manjuu PC 启动器的资源下载流程。

- 不需要图形界面。
- 下载器本身不运行 Windows EXE，不依赖 Wine 或 Proton。
- 使用 Go 标准库实现 HTTP 下载、断点续传、CDN 重试、索引解密、MD5/文件大小校验。
- 不分发原始安装器、`cutils.node`、`app_config.dat` 或游戏资源。

> 当前版本为首个可编译的下载实现。原始启动器的资源索引格式、AES-192-CBC 参数、文件清单字段、切片包/逐文件包、Range 下载、MD5 校验和预下载差异逻辑已从所提供的启动器中还原。首次运行前仍需从原安装器提取启动器配置；真实 API 连通性及服务端签名验收需要在拿到该配置后完成，不能把本地单元测试当成真实服务器验收。

## 1. 构建

要求 Go 1.22 或更新版本。无第三方 Go 依赖。

```bash
go build -trimpath -ldflags='-s -w' -o azurpromilia ./cmd/azurpromilia
```

查看帮助：

```bash
./azurpromilia help
```

切片包自动解压需要系统安装 `7z` 或 `unzip`。如果不安装，下载器仍会下载、合并并校验 `game.zip`，但不会假装解压成功。

## 2. 首次配置：从原始 EXE 提取

`res/app_config.dat` 是启动器配置文件。原版通过 Windows x64 的 `napi-cutils` 原生模块读取，Linux 无法直接加载这个 PE 原生模块。

在 Windows x64 虚拟机中运行：

```powershell
py -3 tools\extract_app_config.py `
  --exe .\AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe `
  --out .\config.json
```

脚本会：

1. 从 EXE 的 PE 资源中提取内部 ZIP；
2. 提取原版 Electron 运行时、`app_config.dat` 和 `cutils.node`；
3. 使用原版 Electron/N-API 模块读取真实配置；
4. 生成本地 `config.json`。

它不会把 `secret_key` 打印到终端，也不会把原生模块复制进 Git 仓库。

如果只想先检查提取是否成功：

```bash
python3 tools/extract_app_config.py --exe ./AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe --work ./.config-extract --extract-only
```

Linux 上的 `--extract-only` 只负责解包；解密配置必须在 Windows x64 上调用原版原生模块。不要手工猜测 `secret_key`。

`config.json` 含有启动器签名密钥，已加入 `.gitignore`。不要上传到公开仓库或发给他人。

## 3. 下载

```bash
./azurpromilia download \
  --config ./config.json \
  --root "$HOME/Games/AzurPromilia" \
  --workers 4
```

默认流程：

1. 请求 Manjuu 启动器配置 API；
2. 请求正式资源和预下载资源配置；
3. 按服务器 `downloadMode` 选择切片包或逐文件资源包；
4. 下载加密索引并按原启动器 AES-192-CBC 参数解密；
5. 解析资源文件大小、MD5、文件名和远程路径；
6. 下载资源，默认 4 并发；
7. 支持 HTTP Range 断点续传、CDN 切换和失败重试；
8. 每个文件落位前校验大小与 MD5；
9. 切片包会按索引顺序合并，校验最终 ZIP 的大小和 MD5；
10. 如果 API 返回预下载补丁，则对比正式索引与预下载索引，只下载发生变化的文件到 `_pre_download/<version>`。

可选参数：

```bash
# 只下载正式资源，不取预下载补丁
./azurpromilia download --no-preload --config ./config.json --root "$HOME/Games/AzurPromilia"

# 下载并校验切片包，但暂不解压
./azurpromilia download --no-extract --config ./config.json --root "$HOME/Games/AzurPromilia"

# 调整并发数
./azurpromilia download --workers 8 --config ./config.json --root "$HOME/Games/AzurPromilia"
```

断点文件使用 `.part` 后缀。进程被中断后重新运行同一命令，会从已有临时文件继续；服务端不支持 Range 时会自动退回完整下载，不会把 200 响应错误地追加到旧文件。

## 4. 完整性检查

```bash
./azurpromilia verify --root "$HOME/Games/AzurPromilia"
```

校验依据是本地保存的原始资源清单，逐项检查文件大小与 MD5。切片模式还会检查所有切片及合并后的 `game.zip`。

MD5 在这里用于匹配官方资源索引，不代表现代密码学意义上的抗碰撞签名。

## 5. 应用预下载补丁

预下载文件先存放在：

```text
<游戏目录>/_pre_download/<version>/
```

下载完成后先执行：

```bash
./azurpromilia verify --root "$HOME/Games/AzurPromilia"
```

确认正式游戏目录已准备好，再应用补丁：

```bash
./azurpromilia apply-preload --root "$HOME/Games/AzurPromilia"
```

也可以显式指定版本：

```bash
./azurpromilia apply-preload --root "$HOME/Games/AzurPromilia" --version VERSION_CODE
```

应用补丁会将差异文件移动到游戏目录并覆盖同名文件。应用后再次执行 `verify`。不要在正式资源尚未下载/解压完成时应用补丁。

## 6. Proton 启动配置参考

下载器不负责配置 Proton，也不会调用 Proton。游戏目录准备好后：

1. 在 Steam 中添加游戏实际的 Windows 主程序作为“非 Steam 游戏”；
2. 在属性的“兼容性”中强制使用你自行选择的 Proton / GE-Proton；
3. 建议为游戏使用独立 CompatData 前缀，避免污染其他游戏；
4. 游戏启动参数和运行库问题单独处理，不要把启动失败误判成资源下载失败。

独立前缀的启动选项形式参考（Steam 安装路径按实际情况调整）：

```bash
STEAM_COMPAT_DATA_PATH="$HOME/Games/compatdata/azurpromilia" %command%
```

具体游戏 EXE 路径以正式资源索引和实际解压结果为准，本项目不猜测入口文件。

## 7. 目录结构

```text
AzurPromilia/
├── <游戏资源文件...>
├── _pre_download/<version>/       # 预下载差异文件
└── .azurpromilia/
    ├── index/                     # 已校验的加密索引文件
    ├── manifests/                 # 下载与校验清单
    ├── slices/<version>/          # 切片包分片
    ├── archives/                  # 合并后的 ZIP
    └── state.json
```

## 8. 当前边界

- 当前阶段只覆盖下载、断点续传、CDN 重试、资源完整性校验、切片合并/解压和预下载差异文件；没有 UI、账号登录 UI、自动 Proton 配置或游戏启动器功能。
- `uid` / `utoken` 可在本地 `config.json` 中按需填写；是否允许匿名预下载以官方 API 实际响应为准。
- 真实服务端验收必须使用从原始 EXE 提取的真实配置进行；测试服务器和本地单元测试不替代这一步。
- 本仓库不包含官方安装器、官方原生模块、官方游戏文件或用户密钥。
