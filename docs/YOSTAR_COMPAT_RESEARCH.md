# Yostar 日本启动器兼容性调查

> 状态：未来兼容计划，当前暂缓；不开发适配器、不运行自动探测。已有静态证据仅作后续参考。  
> 样本来源：日本官方 `https://azurpromilia.jp/PCdownload` 页面解析出的 Yostar 官方下载地址。  
> 当前样本 SHA-256：`dde29b4fa4a6eabfb664838c807460e65746f590f1a51754a33d976763bb1e3f`

## 1. 未来为什么值得研究

当前 Manjuu / SC000 Linux 下载器已经把下载、Range、MD5、SLICE/REALITY、preload、update 等核心能力拆成了相对独立的 Go 逻辑。

如果 Yostar 日本服只是：

- 外层安装器不同；
- metadata/bootstrap 不同；
- 实际游戏下载索引与 CDN 协议相同或相近；

那么不需要维护第二套下载器，只需要增加一个 Yostar metadata adapter，继续共用现有下载核心。

因此当前目标不是“再逆一个完整项目”，而是先判断协议共用程度。

## 2. 已确认的安装器结构

GitHub Actions 静态调查从官方页面下载了当前日本包：

- host：`launcher-pkg-azupro-jp.yo-star.com`
- 文件大小：179,893,048 bytes
- SHA-256：`dde29b4fa4a6eabfb664838c807460e65746f590f1a51754a33d976763bb1e3f`

PE/版本信息：

- 32-bit Windows GUI PE；
- `FileVersion: 1.8.1.92`；
- `ProductVersion: 1.8.1.92`；
- `CompanyName: Yostar, Inc.`；
- `ProductName: AZUPRO_JP_Gamelauncher`。

7-Zip 能直接识别其主体：

- `NSIS-3 Unicode`；
- NSIS payload 从约 193,536 字节处开始；
- 外层不是旧 SC000 样本使用的嵌入 ZIP 结构。

这解释了为什么现有 `tools/bootstrap.py` 对它报：

`embedded launcher ZIP not found`

这个错误是封装格式不兼容，不代表 Electron 或游戏下载协议完全不同。

## 3. Electron 证据

NSIS 中可以直接解出完整 Electron/Chromium 运行组件，包括：

- `LICENSE.electron.txt`
- `LICENSES.chromium.html`
- `chrome_100_percent.pak`
- `chrome_200_percent.pak`
- `d3dcompiler_47.dll`
- `ffmpeg.dll`
- `icudtl.dat`
- `libEGL.dll`
- `libGLESv2.dll`
- `resources.pak`
- `snapshot_blob.bin`
- `v8_context_snapshot.bin`
- `vulkan-1.dll`
- 多语言 Chromium locale

关键文件：

| 文件 | 解包后大小 |
|---|---:|
| `resources/app.asar` | 11,919,408 bytes |
| `launcher-installer.exe` | 136,439,680 bytes |
| `app-32.7z` | 76,119,947 bytes |

因此可以确认：**Yostar 日本启动器本身仍然是 Electron 系启动器**，只是外层安装/自更新方式与 Manjuu / SC000 不同。

## 4. 当前还不能下的结论

对外层 EXE 做裸字符串扫描时，没有直接看到：

- `api-grp.manjuu.com`
- `api-grp-l2.manjuu.com`
- `packageconfig`
- `gameversion`
- `downloadMode`
- `gamePackageList`
- `cdnBaseUrls`
- `launchEntry`
- `napi-cutils`
- `SC000`

但外层 NSIS 数据是压缩的，所以这**不能证明 Yostar 使用不同协议**。真正有价值的目标是解出的 `app.asar`、`launcher-installer.exe` 和 `app-32.7z`。

## 5. 恢复该方向时的静态调查清单

下一步只回答这些问题：

1. `resources/app.asar` 中的 main/package 配置是什么；
2. 是否仍存在 packageconfig/gameversion API；
3. API host 是 Manjuu、Yostar 自有域名还是区域网关；
4. 是否仍存在 SLICE/REALITY 或等价的两种资源包类型；
5. index 是否仍是 Base64 + AES-CBC，字段格式是否兼容；
6. `launcher-installer.exe` 是真正 bootstrap，还是单纯自更新辅助程序；
7. `app-32.7z` 中是否是最终 launcher 安装树；
8. Yostar 的版本文件、游戏目录和 preload/update 流程是否能映射到现有 Go workflow。

这一步只做静态解析，不运行安装器业务逻辑，不登录账号。

## 6. 预期代码结构

若协议层相同或大部分相同，优先做成：

```text
CLI / workflow
      |
common downloader
Range / retry / CDN / MD5 / ZIP / verify
preload / update / repair
      |
+------------------------+------------------------+
| Manjuu / SC000 adapter | Yostar / JP adapter    |
| bootstrap + metadata   | bootstrap + metadata   |
+------------------------+------------------------+
```

只有当 Yostar 的游戏下载协议本身与 SC000 完全不同，才考虑独立 backend。

当前主线优先完成 Manjuu / SC000 的 SLICE 全量安装、预下载和跨版本升级验收。Yostar 方向暂时冻结；未来恢复时先按本清单做静态分析，不直接开始大规模重构。

## 2026-11-27 后续进展

此文是历史调查记录；冻结状态已经解除。日服实验性下载实现及实服证据见 [YOSTAR_JP_DOWNLOAD.md](YOSTAR_JP_DOWNLOAD.md)。
