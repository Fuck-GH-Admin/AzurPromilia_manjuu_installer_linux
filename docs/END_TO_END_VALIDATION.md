# 完整链路实服验收记录

> 状态：未完成全量安装验收  
> 最后整理：2026-11-25  
> 规则：只记录真实官方 API/CDN 请求和真实本地安装结果。单元测试、fixture 和本地 HTTP 测试必须单独标注，不能替代上游证据。

## 1. 当前已确认的官方实测基线

来源：`docs/LOCAL_TESTING.md` 第 10 节。

| 项目 | 记录 | 证据状态 |
|---|---|---|
| 原始安装器 | `AzurPromilia_manjuu_installer_1.0.1.1_35_SC000_202610282107.exe` | 样本 SHA-256 已记录 |
| 样本 SHA-256 | `cf4b1f6233af2f2cb2d1c0de12f54d07606c3c84c829390b6caccfb2d0d1f04b` | 已核对 |
| 匿名 packageconfig | 官方 API 返回 `code=0`；记录 released `1.0.0.30` | 真实请求通过 |
| REALITY 索引 | 15,024,044 字节；40,457 个文件、29 个目录 | 真实 CDN 下载、MD5、解密、解析通过 |
| REALITY 小文件 | 探测资源 4 字节 | 真实 CDN 下载、大小、MD5 通过 |
| SLICE 索引 | 148,184 字节；752 个分片 | 真实 CDN 下载、MD5、解密、解析通过 |
| SLICE 小文件 | 探测资源 45,275,761 字节 | 真实 CDN 下载、大小、MD5 通过 |
| CDN | `syncstation.manjuu.com`、`l2-syncstation.manjuu.com` | 两个 CDN 均完成小文件探测 |
| Range 续传 | 续传起点 42,991,616；官方返回 HTTP 206 和匹配 Content-Range | 真实 CDN 续传通过 |
| 当前 downloadMode | 旧记录未抄录具体数值 | **待重新运行 info 确认** |
| preload 版本 | 旧记录未保留版本号与是否存在的明确结论 | **待重新运行 info 确认** |

注意：以上只证明索引和小文件探测，不证明完整安装。

## 2. 尚未通过的端到端项目

| 项目 | 状态 | 完成条件 |
|---|---|---|
| `info` 当前元数据刷新 | 待重新实测 | 记录本轮版本、downloadMode、preload、包类型与 API 响应结果 |
| REALITY 全量安装 | 未验收 | 全部 manifest 文件下载/复用，最终 verify 0 失败 |
| SLICE 全量安装 | 未验收 | 全部分片校验、合并 ZIP MD5、解压后 CRC32/大小全部通过 |
| Repair | 未验收 | 在可丢弃目录破坏一个受追踪文件，repair 恢复 MD5，最终 verify 通过 |
| Preload 差异下载 | 未验收 | 官方存在不同 preload 版本；仅下载真实差异文件，暂存 verify 通过 |
| Update | 未验收 | 存在真实新旧版本差异；预下载复用、文件补齐、旧文件清理和最终 verify 通过 |
| 游戏启动 / Proton | 未验收 | 确认真实启动入口与参数，再单独测试 Proton |

## 3. 每次真实测试记录模板

每轮测试新增一条，不覆盖旧记录。

| 字段 | 记录值 |
|---|---|
| 测试日期与本地时区 | 待填写 |
| 系统 / 架构 | 待填写 |
| Wine / Go / CLI 版本 | 待填写 |
| 原始安装器 SHA-256 | 待填写 |
| 官方 API 主站 / 备用站 | 待填写 |
| released versionCode / versionName | 待填写 |
| preload versionCode / versionName | 待填写或官方无 preload |
| downloadMode | 待填写 |
| 实际 packageType | 待填写 |
| 操作 | info / install / verify / repair / preload / update |
| manifest 文件数 / 目录数 | 待填写 |
| 已校验文件数 / 失败数 | 待填写 |
| 理论下载字节 / 实际传输字节 | 待填写 |
| 复用的本地或 preload 字节 | 待填写 |
| 运行耗时 | 待填写 |
| 最终版本标记 | 待填写 |
| 错误与恢复情况 | 待填写 |
| 脱敏日志或 CI 链接 | 待填写 |

## 4. 证据要求

- 官方 API：记录请求时间、目标 host、HTTP 状态、业务 code 和必要的版本字段。
- 官方 CDN：记录 host、资源大小、MD5、HTTP Range/Content-Range（如适用）；不保存临时签名 URL。
- 全量安装：记录 manifest 总数、通过数、失败数、总字节和最终 verify。
- 更新：同时保留更新前后版本、旧/新 manifest 的差异摘要、清理文件列表和最终 verify。
- Repair：记录测试文件原始/损坏/修复后的 MD5。
- Preload：记录 released/preload 的版本与差异文件数，区分跳过、已缓存、实际下载。
- 单元测试与 CI：单独记录测试名称、提交 SHA 和 workflow run，不标注为官方实服证据。

禁止提交原始 `packageconfig.json`、临时 CDN URL、账号凭据、Cookie、token、签名头、secretKey 或个人设备标识。
