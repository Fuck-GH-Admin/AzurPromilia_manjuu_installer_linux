# AzurPromilia Manjuu Installer Linux

非官方 Linux 原生 CLI。已编写 CDN 下载、索引解析、断点续传、MD5/大小验证等核心代码，但**尚未完成真实官方 CDN 验收**。当前为开发版，不应视为开服即用的正式发行版。

## 当前状态与本地测试

- [实现状态、已知缺口与验收标准](docs/IMPLEMENTATION_STATUS.md)
- [本地测试步骤与问题反馈清单](docs/LOCAL_TESTING.md)

请先阅读这两份文档，再开始真实下载。当前 API bootstrap 仅为实验性匿名请求；预下载补丁、升级/修复、自动下载模式选择均未完成。

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

引导器尝试调用原版 Electron/N-API 组件生成匿名包清单请求；请求头尚未完整复现，且没有账号登录支持，暂未实服验收。若官方接口拒绝匿名请求，停止并报告错误，不尝试绕过。下载器本身仍完全原生 Linux，不依赖 Wine。亦可直接导入自己合法获得的原始包清单。不要公开 packageconfig.json（其中可能有临时 CDN URL）。

## 下载

将你正常使用的官方启动器获得的 packageconfig JSON 响应保存为本地 packageconfig.json。不要上传包含账号、令牌或签名的请求信息。Go 下载器本身尚未实现纯 Go 官方 API 签名；一次性 bootstrap 借用原版 Windows 原生模块进行实验性匿名签名请求，不支持账号登录或 token。

```sh
./azurpromilia download --installer /path/to/original.exe --packageconfig packageconfig.json --root "$HOME/Games/AzurPromilia" --workers 4
./azurpromilia verify --root "$HOME/Games/AzurPromilia"
```

默认选择 released 的 packageType=2 逐文件包，--slice 选择 type=1 切片 ZIP 包；--target preload 选择预下载完整资源索引（独立目录，不自动应用）；--no-extract 仅合并切片并校验 ZIP。原始安装器用于本地读取索引解码参数，不提交原始二进制或参数。每个文件校验后才重命名，失败退出非零；重复运行会跳过已通过验证的文件。切片模式 verify 验证切片和合并 ZIP，并按已验证 ZIP 中的 CRC32 与大小检查解压文件；仅下载 ZIP 时使用 verify --no-extract。预下载校验使用 verify --target preload --version VERSION_CODE。

## 实服探测

实服探测工作流只是可手动触发的验证工具，目前没有成功运行记录；在用户本地验收前，不要将其视为通过。


Actions 中的 Official CDN live probe 支持手动输入该原始安装器的官方 HTTPS URL；先核对样本 SHA256，再使用官方组件取得包清单、下载并校验实际索引及一个不超过 64 MiB 的资源。不会上传游戏文件、包清单或密钥。它与完整下载命令独立，只是实服兼容性检查。尚未运行成功，不应视为已验收。

## 证据与限制

样本 SHA256 cf4b1f6233af2f2cb2d1c0de12f54d07606c3c84c829390b6caccfb2d0d1f04b。内部 ZIP 位于偏移 12453620，包含 resources/app.asar、res/app_config.dat 和 Windows cutils.node。main.js 的 parseGameIndex 是 5 列记录（大小、MD5、相对路径、CDN 路径）；parseInstallIndex 使用 3 列 ZIP 记录及 5 列切片记录。索引为 Base64 编码的 AES-CBC，先按 API 返回的大小和 MD5 验证密文。原始 API 入口 /pc-launcher/client/gameversion/packageconfig，签名由原版 Windows 原生模块处理，尚未完成 Linux 签名兼容或实服验证。MD5 用于兼容官方校验，不是可信数字签名。测试服务器只验证下载行为，不能证明官方 CDN 可访问。尚未实现自动版本发现、预下载差异比较/合并、旧文件清理。

## Proton

Steam 添加实际游戏 EXE 为非 Steam 游戏，设置工作目录为游戏目录，兼容性选择 Proton/GE-Proton。可用 STEAM_COMPAT_DATA_PATH="$HOME/Games/compatdata/azurpromilia" %command% 隔离前缀。实际游戏 EXE 和运行兼容性待实测；不修改反作弊或游戏代码。
