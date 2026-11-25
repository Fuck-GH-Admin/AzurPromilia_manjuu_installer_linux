# 明早实服测试要点

目标：先把当前 **Manjuu / SC000** 下载器真实跑通。不要顺手测试日本 Yostar 包，不要混渠道结论。

## 1. 测试前

- 使用已经逆向过的原始安装器：
  \`AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe\`
- SHA-256：
  \`cf4b1f6233af2f2cb2d1c0de12f54d07606c3c84c829390b6caccfb2d0d1f04b\`
- 使用独立、可丢弃的空目录，例如：
  \`$HOME/Games/AzurPromilia-test\`
- 确认磁盘空间充足。新版本会在真正下载前打印：
  - 预计还需要的空间；
  - 当前文件系统可用空间。
- 不需要提供账号、Cookie、token 或设备标识。

## 2. 第一步：只看当前服务器状态

\`\`\`bash
./azurpromilia info \
  --installer /path/to/AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe \
  --root "$HOME/Games/AzurPromilia-test"
\`\`\`

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

\`\`\`bash
./azurpromilia install \
  --installer /path/to/AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe \
  --root "$HOME/Games/AzurPromilia-test" \
  --workers 4
\`\`\`

重点看：

- 实际选择的 packageType 是否与 downloadMode 一致；
- 磁盘空间预检查是否合理；
- 下载过程中是否出现 CDN 切换；
- 若某个文件最终失败，后续新任务是否停止派发；
- Ctrl+C 中断后再次执行能否利用 \`.part\` 续传；
- SLICE 模式若被选中，分片总大小是否与最终 ZIP 声明一致，合并和解压是否通过。

不要为了测试异常而中断第一次完整安装。先争取拿到一次干净的成功样本。

## 4. 第三步：全量校验

安装成功后：

\`\`\`bash
./azurpromilia verify \
  --root "$HOME/Games/AzurPromilia-test"
\`\`\`

要求：

- 最终失败数为 0；
- REALITY：全部 manifest 文件大小 + MD5 通过；
- SLICE：解压后的大小 + CRC32 通过；
- \`_index/version\`、本地 manifest 与安装版本一致。

如果 verify 有失败，不要马上重装。先保留：

- 失败文件相对路径；
- 预期大小 / MD5 或 CRC32；
- 实际文件大小；
- 最后使用的 CDN / HTTP 错误；
- 是否存在同名 \`.part\`。

## 5. 第四步：Repair

只在这个可丢弃测试目录中做。

任选一个 **manifest 明确追踪、体积不太大的普通资源文件**：

1. 先记录它的 MD5；
2. 删除或改坏该文件；
3. 执行：

\`\`\`bash
./azurpromilia repair \
  --installer /path/to/AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe \
  --root "$HOME/Games/AzurPromilia-test" \
  --workers 4
\`\`\`

4. 再运行一次 \`verify\`。

通过条件：

- 只重新下载损坏/缺失资源；
- 修复后的 MD5 恢复；
- 其他已校验文件不被重复下载；
- 最终 verify 0 失败。

## 6. Preload / Update 何时测

只有 \`info\` 明确显示真实 preload 或新的 released 版本时才继续。

若存在 preload：

\`\`\`bash
./azurpromilia preload \
  --installer /path/to/original.exe \
  --root "$HOME/Games/AzurPromilia-test" \
  --workers 4
\`\`\`

记录：

- preload version；
- 差异文件数量；
- 理论差异字节；
- 实际下载字节；
- 已缓存/跳过数量。

等该 preload 真正变成 released 后再执行：

\`\`\`bash
./azurpromilia update \
  --installer /path/to/original.exe \
  --root "$HOME/Games/AzurPromilia-test" \
  --workers 4

./azurpromilia verify \
  --root "$HOME/Games/AzurPromilia-test"
\`\`\`

若服务器没有真实版本变化，就先不测 update。不要手工改 versionCode 来伪造“升级成功”。

## 7. 最需要回传的东西

如果出现问题，优先回传：

- 完整命令；
- 从命令开始到错误结束的终端日志；
- \`info\` 的版本 / downloadMode / preload 摘要；
- 失败文件相对路径；
- 是否重跑、是否有 \`.part\`；
- 文件系统剩余空间；
- 程序退出码。

不要上传完整 \`packageconfig.json\`、临时 CDN URL、签名、secretKey 或任何账号凭据。

---

# Yostar 日本下载器：后续兼容性调查

GitHub Actions 已从日本官方 \`azurpromilia.jp/PCdownload\` 解析并下载当前 Yostar 安装器：

- 官方包 host：\`launcher-pkg-azupro-jp.yo-star.com\`
- 大小：179,893,048 bytes
- SHA-256：
  \`dde29b4fa4a6eabfb664838c807460e65746f590f1a51754a33d976763bb1e3f\`
- 当前 Manjuu bootstrap 结果：
  \`embedded launcher ZIP not found\`

这说明它不是现有 SC000 安装器的同一外层封装。

后续只做一轮静态结构调查，先回答以下问题：

1. 安装器外层是什么：NSIS、Inno、MSI、7z SFX、Electron Builder，还是 Yostar 自研封装？
2. 是否仍包含 Electron / Node / ASAR 启动器？
3. 是否仍使用 Manjuu 的 \`api-grp.manjuu.com\`、相同 packageconfig API、SLICE/REALITY 索引格式？
4. 是否只是“外层安装器不同，内部游戏下载协议相同”？
5. AES/index 参数、CDN 域名、渠道字段是否不同？
6. 游戏目录、版本文件和更新流程是否与 SC000 接近？

优先目标不是维护两套代码，而是判断能否抽象成：

\`\`\`
CLI / workflow
      |
common download + verify + update core
      |
+----------------------+----------------------+
| Manjuu / SC000       | Yostar / JP          |
| metadata adapter     | metadata adapter     |
+----------------------+----------------------+
\`\`\`

如果 Yostar 最终只是 metadata/bootstrap 不同，就只增加一个适配层，共用现有 downloader、Range、MD5、SLICE/REALITY、preload/update 代码。

如果游戏下载协议本身也完全不同，再决定是否值得支持；在调查完成前不重构当前 Manjuu 主线。
