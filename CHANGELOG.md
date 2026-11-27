# 更新日志

本项目的公开版本历史。日期为发布日；条目面向使用者视角，不涉及内部过程。

## [v1.1.1] - 2026-11-27

### 变更
- DNSPod 记录的 TTL 低于 600 秒（免费版下限）时，创建/更新前输出非阻断告警，提前暴露可能被 API 拒绝或静默改写的配置。

### 修复
- 读取与写入 DNSPod 解析线路（`默认` / `境外`）时做双向映射：此前远端线路未回填到内部状态，`oversea` 记录会被当作默认线路，规划时反复判为新增，更新也可能落到错误线路。
- 规划时排除 DNSPod 自动创建的 `@ NS` 系统记录：此前向新 zone 首次下发时可能尝试删除该记录。
- DNS 远端记录列举失败不再静默当作「没有记录」：规划 `dns deploy`（含 `--dry-run`）时对瞬时错误（超时、连接重置、云厂商限流/内部错误）自动重试，重试后仍失败则中止本次规划并输出失败域名与底层原因；无配置记录的域名仅告警跳过，不阻塞整批规划。此前失败当空会把配置中的全部记录误判为新增，存在误下发风险。

## [v1.1.0] - 2026-11-24

### 新增
- prod 部署自动保留上一版镜像为 `<repo>:prod-prev`，保证任何时刻都有可回滚目标；回滚部署（目标为 `:prod-prev`）优先使用本地保留镜像并跳过 `compose pull`。

### 变更
- 移除 frp 隧道 `tunnel-server` / `tunnel-client` 服务类型与隧道路由生成；配置中仍声明这两类服务的环境需在升级前调整。
- `server prune` 的 `image` / `all` 过滤不再使用 `-a`：仅清理 dangling 镜像，带标签镜像（含 `:prod-prev` 回滚镜像）及其层保留，避免回滚目标被清理。

### 修复
- 部署告警（镜像保留打标签失败、镜像 pull 失败）在 CLI deploy 输出中可见，不再静默。

## [v1.0.35] - 2026-11-15

### 新增
- 网关 SSL 明文 http 证书端点放行开关透传：`ssl.allow_insecure_http`（gate `ssl.remote.allow_insecure_http`）——`mode: remote` 且 endpoint 含明文 http 时必须显式 `true`，否则实体层校验直接报错。
- 网关 WAF CRS 降级双钥匙确认透传：`waf.crs_degrade_acknowledged`（gate `waf.crs.degrade_acknowledged`）——`crs_failure_policy: degrade` 时必须显式 `true`，实体层校验直接报错，不会等到网关启动后才暴露。非 degrade 配置不生成该键，产物逐字节不变。

### 修复
- `local` 环境网关的 shipper 不变式校验提示补位：`type: gateway` 缺 `tracker_shipper` 段的部署被拦截时，按实体显式 `enabled: false` 豁免形态下发。

## [v1.0.34] - 2026-11-14

### 移除（破坏性变更）
- 移除 uptime-kuma 服务类型、生成器与同步 agent：不再生成 `uptime-kuma` 容器及 `<服务名>-agent` 派生容器，`service cleanup` 的孤儿判定同步去掉 uptime-kuma 派生名放行。配置中仍有 `type: uptime-kuma` 的服务需在升级前迁移或删除。

## [v1.0.33] - 2026-11-08

### 新增
- 开源首个公开发布：基于 YAML 的基础设施即代码部署工具（CLI / TUI），覆盖服务器纳管、业务与基础设施服务编排、网关配置生成、DNS 与证书管理等场景。
- mesh 组网全局设置：`userdata/<env>/mesh.yaml` 新增 `mesh.domain_zone` 配置项，mesh 域名后缀判定（`.{节点}.v-{env}.{zone}`）由硬编码改为按环境配置；未配置时相关自动行为关闭。

### 变更
- 网关生成器的 forwarded_for 自动打标与二级链路 HSTS 检查改为依赖上述配置项，配置后行为与此前版本一致。
