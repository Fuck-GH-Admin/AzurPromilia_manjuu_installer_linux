# AzurPromilia Manjuu Installer Linux

非官方 Linux 原生 CLI。真实 CDN 下载、索引解析、断点续传、MD5/大小验证；不运行 Windows 下载器，不分发游戏或官方二进制。当前为**需导入官方包配置的开发版**，不是已完成自动 API 对接的发行版。

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

引导器只调用原版 Electron/N-API 组件生成正常匿名包清单请求，调用官方 API，不输出签名密钥；暂未实服验收。若官方接口拒绝匿名请求，停止并报告错误，不尝试绕过。下载器本身仍完全原生 Linux，不依赖 Wine。亦可直接导入自己合法获得的原始包清单。不要公开 packageconfig.json（其中可能有临时 CDN URL）。

## 下载

将你正常使用的官方启动器获得的 packageconfig JSON 响应保存为本地 packageconfig.json。不要上传包含账号、令牌或签名的请求信息。本程序暂未实现官方原生 API 签名，不会伪造账号或绕过授权。

```sh
./azurpromilia download --installer /path/to/original.exe --packageconfig packageconfig.json --root "$HOME/Games/AzurPromilia" --workers 4
./azurpromilia verify --root "$HOME/Games/AzurPromilia"
```

默认选择 released 的 packageType=2 逐文件包，--slice 选择 type=1 切片 ZIP 包；--target preload 选择预下载完整资源索引（独立目录，不自动应用）；--no-extract 仅合并切片并校验 ZIP。原始安装器用于本地读取索引解码参数，不提交原始二进制或参数。每个文件校验后才重命名，失败退出非零；重复运行会跳过已通过验证的文件。切片模式 verify 验证切片及合并 ZIP，不等价于验证已解压的逐文件内容。

## 证据与限制

样本 SHA256 cf4b1f6233af2f2cb2d1c0de12f54d07606c3c84c829390b6caccfb2d0d1f04b。内部 ZIP 位于偏移 12453620，包含 resources/app.asar、res/app_config.dat 和 Windows cutils.node。main.js 的 parseGameIndex 是 5 列记录（大小、MD5、相对路径、CDN 路径）；parseInstallIndex 使用 3 列 ZIP 记录及 5 列切片记录。索引为 Base64 编码的 AES-CBC，先按 API 返回的大小和 MD5 验证密文。原始 API 入口 /pc-launcher/client/gameversion/packageconfig，签名由原版 Windows 原生模块处理，尚未完成 Linux 签名兼容或实服验证。MD5 用于兼容官方校验，不是可信数字签名。测试服务器只验证下载行为，不能证明官方 CDN 可访问。尚未实现自动版本发现、预下载差异比较/合并、旧文件清理。

## Proton

Steam 添加实际游戏 EXE 为非 Steam 游戏，设置工作目录为游戏目录，兼容性选择 Proton/GE-Proton。可用 STEAM_COMPAT_DATA_PATH="$HOME/Games/compatdata/azurpromilia" %command% 隔离前缀。实际游戏 EXE 和运行兼容性待实测；不修改反作弊或游戏代码。
