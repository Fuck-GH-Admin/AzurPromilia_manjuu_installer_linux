# v0.1.0-preview.1 发布验证报告

验证日期为 2026-11-26。验收覆盖 Linux 下载器包、官方 SC000 启动器自动获取、客户端包清单获取以及一个真实游戏资源的下载和校验。实服探测使用临时目录，游戏安装样本仍为先前验证的 `1.0.0.30`。

## 结果

| 项目 | 环境与结果 |
|---|---|
| 最新 main 的实服探测 | 源码基线 `f53f82f`，Ubuntu 26.04.1 x86-64，Go 1.27.0、Python 3.14.4、Wine 10.0 |
| amd64 预编译包 | 从该提交的 GitHub Actions 构建下载，压缩包和二进制 SHA-256 校验通过。解压后的 CLI 在独立目录重新取得官方包清单，并返回 `LIVE PROBE OK` |
| 官方启动器冷缓存 | 从空缓存直连官网发现接口和官方 CDN，自动下载 SC000 启动器 `1.0.2.2`，共 190,835,304 字节。结构和 SHA-256 校验通过 |
| 官方客户端包清单 | `released=1.0.0.30`，`versionName=1.0.0`，`downloadMode=2`（REALITY），同时提供 packageType 1 和 2，启动入口为 `AzurPromilia.exe` |
| 官方 REALITY 下载 | 索引 15,024,044 字节。索引解密解析后下载了 4 字节真实游戏资源，索引和资源的大小与 MD5 均通过 |
| arm64 下载器 | 本机交叉编译为 Linux ELF64 AArch64，用 QEMU 用户模式 10.2.1 运行。导入上述官方启动器和包清单后，直连 CDN，下载并校验同版本索引和资源，返回 `LIVE PROBE OK` |
| 本机编译与静态检查 | `go test -race -count=1 ./...`、`go vet ./...` 及 Python / Node.js 语法检查通过 |
| 双架构发布打包 | 在本机运行发布工作流的打包步骤，两种压缩包和内含二进制均通过 SHA-256 校验。amd64 在本机、arm64 在 QEMU 下均可运行 `help` |

官方启动器 SHA-256 为 `0a1d1b97947838146c7b8f3f0d27dbba79e60372d043f60cd5401f2979531282`。[用于实测的 amd64 构建](https://github.com/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux/actions/runs/45800041712)

## 复测

在解压后的下载器目录运行。无桌面会话时可用 `xvfb-run -a` 运行 `info`。

~~~bash
test_dir=$(mktemp -d)
export XDG_CACHE_HOME="$test_dir/cache"
export WINEPREFIX="$test_dir/wine"
export WINEARCH=win64

./azurpromilia info --root "$test_dir/game"
./azurpromilia download \
  --root "$test_dir/game" \
  --packageconfig "$test_dir/game/.azurpromilia/packageconfig.json" \
  --probe
~~~

本轮实测止于小文件探测。已有的 amd64 REALITY 全量安装、40,457 个文件校验与单文件修复结果记录在项目的实服验收文档。本轮 arm64 测试使用导入的包清单；自动引导需要可执行官方 x86-64 Windows 启动器的环境。SLICE 全量安装、预下载和跨版本更新仍待实服验收。[全量安装实测](NEXT_LIVE_TEST.md)
