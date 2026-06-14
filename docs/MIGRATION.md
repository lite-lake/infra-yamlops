# 迁移指南

本文档帮助您从旧版本的 YAMLOps 命令迁移到 v1.0.0 的新命令结构。

## 概述

v1.0.0 进行了全面的命令重组，主要变化：

1. **四大模块**：命令统一为 `config`、`dns`、`server`、`service` 四个模块
2. **统一执行模式**：所有变更命令遵循 `Plan → Confirm → Execute` 三阶段
3. **参数统一**：`--yes` 替代 `--auto-approve`，`--force` 仅在需要时支持
4. **`-e` 必填**：所有命令强制要求 `-e` 参数（`--help` 除外）
5. **取消 `list` 命令**：统一使用 `show` + `--detail`
6. **引入 `--dry-run`**：替代独立的 `plan` 命令

---

## 命令迁移对照表

### 核心命令

| 旧命令 | 新命令 | 说明 |
|--------|--------|------|
| `yamlops cli plan -e prod` | `yamlops cli service deploy -e prod --dry-run` | Plan 改为 dry-run |
| `yamlops cli apply -e prod` | `yamlops cli service deploy -e prod --yes` | Apply 改为 deploy --yes |
| `yamlops cli validate -e prod` | `yamlops cli service validate -e prod` + `yamlops cli server validate -e prod` + `yamlops cli dns validate -e prod` + `yamlops cli config validate -e prod` | 拆分为各模块 validate |
| `yamlops cli list -e prod` | `yamlops cli service show -e prod` | list 改为 show |
| `yamlops cli show -e prod` | `yamlops cli service show -e prod --detail` | show 加 --detail |
| `yamlops cli clean -e prod` | `yamlops cli service cleanup -e prod --yes` | clean 改为 cleanup |

### 环境管理命令

| 旧命令 | 新命令 | 说明 |
|--------|--------|------|
| `yamlops cli env check -e prod` | `yamlops cli server setup -e prod --dry-run` | env check 改为 server setup --dry-run |
| `yamlops cli env sync -e prod` | `yamlops cli server setup -e prod --yes` | env sync 改为 server setup --yes |
| `yamlops cli server check -e prod` | `yamlops cli server setup -e prod --dry-run` | server check 改为 server setup --dry-run |
| `yamlops cli server sync -e prod` | `yamlops cli server setup -e prod --yes` | server sync 改为 server setup --yes |

### 应用管理命令

| 旧命令 | 新命令 | 说明 |
|--------|--------|------|
| `yamlops cli app plan -e prod` | `yamlops cli service deploy -e prod --dry-run` | app plan 合并到 service |
| `yamlops cli app apply -e prod --auto-approve` | `yamlops cli service deploy -e prod --yes` | app apply 合并到 service |
| `yamlops cli app list biz -e prod` | `yamlops cli service show -e prod --type biz` | app list 改为 service show |
| `yamlops cli app list infra -e prod` | `yamlops cli service show -e prod --type infra` | app list 改为 service show |
| `yamlops cli app show biz api-server -e prod` | `yamlops cli service show -e prod --type biz --detail` | app show 改为 service show --detail |

### DNS 管理命令

| 旧命令 | 新命令 | 说明 |
|--------|--------|------|
| `yamlops cli dns plan -e prod` | `yamlops cli dns deploy -e prod --dry-run` | dns plan 改为 dns deploy --dry-run |
| `yamlops cli dns apply -e prod --auto-approve` | `yamlops cli dns deploy -e prod --yes` | dns apply 改为 dns deploy |
| `yamlops cli dns list -e prod` | `yamlops cli dns show -e prod` | dns list 改为 dns show |
| `yamlops cli dns show domain example.com -e prod` | `yamlops cli dns show -e prod --domain example.com` | 参数位置调整 |

### 配置管理命令

| 旧命令 | 新命令 | 说明 |
|--------|--------|------|
| `yamlops cli config list -e prod` | `yamlops cli config show isps -e prod` | config list 改为 config show |
| `yamlops cli config list secrets -e prod` | `yamlops cli config show secrets -e prod` | config list 改为 config show |
| `yamlops cli config show isp aliyun -e prod` | `yamlops cli config show isps aliyun -e prod` | 参数位置调整 |

---

## 参数迁移对照表

| 旧参数 | 新参数 | 适用命令 |
|--------|--------|---------|
| `--auto-approve` | `--yes` | 所有变更命令 |
| `--check-only` | `--dry-run` | server setup |
| `--sync-only` | `--yes` | server setup |
| `--biz` | `--type biz` | service 子命令 |
| `--infra` | `--type infra` | service 子命令 |
| `--service <name>` | 不支持，使用 `--type` 或 TUI 逐项选择 | - |

---

## 脚本迁移示例

### 部署服务

**旧脚本**：
```bash
#!/bin/bash
yamlops cli plan -e prod --service api
yamlops cli apply -e prod --service api --auto-approve
```

**新脚本**：
```bash
#!/bin/bash
yamlops cli service deploy -e prod --type biz --yes
```

### 环境同步

**旧脚本**：
```bash
#!/bin/bash
yamlops cli env check -e prod
yamlops cli env sync -e prod --auto-approve
```

**新脚本**：
```bash
#!/bin/bash
yamlops cli server setup -e prod --dry-run  # 检查差异
yamlops cli server setup -e prod --yes       # 直接同步
```

### 验证配置

**旧脚本**：
```bash
#!/bin/bash
yamlops cli validate -e prod  # 一次性验证所有配置
```

**新脚本**：
```bash
#!/bin/bash
# 分别验证各模块配置
yamlops cli service validate -e prod
yamlops cli server validate -e prod
yamlops cli dns validate -e prod
yamlops cli config validate -e prod
```

### DNS 管理

**旧脚本**：
```bash
#!/bin/bash
yamlops cli dns pull domains -e prod -i aliyun --auto-approve
yamlops cli dns pull records -e prod -d example.com --auto-approve
yamlops cli dns plan -e prod
yamlops cli dns apply -e prod --auto-approve
```

**新脚本**：
```bash
#!/bin/bash
yamlops cli dns pull domains -e prod --isp aliyun --yes
yamlops cli dns pull records -e prod --domain example.com --yes
yamlops cli dns deploy -e prod --domain example.com --yes
```

---

## CI/CD 集成迁移

### GitHub Actions

**旧配置**：
```yaml
- name: Deploy services
  run: |
    yamlops cli plan -e ${{ env.ENV }}
    yamlops cli apply -e ${{ env.ENV }} --auto-approve
```

**新配置**：
```yaml
- name: Deploy services
  run: |
    yamlops cli service deploy -e ${{ env.ENV }} --type biz --yes
```

### GitLab CI

**旧配置**：
```yaml
deploy:
  script:
    - yamlops cli plan -e $CI_ENVIRONMENT_NAME
    - yamlops cli apply -e $CI_ENVIRONMENT_NAME --auto-approve
```

**新配置**：
```yaml
deploy:
  script:
    - yamlops cli service deploy -e $CI_ENVIRONMENT_NAME --type biz --yes
```

---

## 迁移检查清单

- [ ] 更新所有脚本中的旧命令为新命令
- [ ] 替换 `--auto-approve` 为 `--yes`
- [ ] 替换 `--check-only` / `--sync-only` 为 `--dry-run` / `--yes`
- [ ] 替换 `--biz` / `--infra` 为 `--type biz` / `--type infra`
- [ ] 检查 BizService 与 InfraService 是否存在同名（新约束禁止同名，`service validate` 会检测）
- [ ] 更新 CI/CD 配置文件
- [ ] 更新文档和 README
- [ ] 通知团队成员命令变更
- [ ] 测试所有新命令在目标环境正常工作

---

## 删除的命令

以下命令在 v1.0.0 中被删除，不再可用：

| 命令 | 原因 | 替代方案 |
|------|------|---------|
| `plan` | 被 `--dry-run` 替代 | `service deploy --dry-run` / `dns deploy --dry-run` / `server setup --dry-run` |
| `apply` | 功能重叠 | `service deploy` |
| `list` | 统一为 `show` | `server show` / `service show` / `dns show` |
| `show`（根级） | 职责拆分到各模块 | `server show` / `service show` / `dns show` / `config show` |
| `clean` | 与 service cleanup 重复 | `service cleanup` |
| `env` | 与 server 重复 | `server setup` |
| `app` | 与 service 重叠 | `service show/deploy` |
| `validate`（根） | 拆分到各模块 | `server validate` / `service validate` / `dns validate` / `config validate` |
| `server check` | 合并到 `server setup` | `server setup --dry-run` |
| `server sync` | 合并到 `server setup` | `server setup --yes` |
| `dns apply` | 重命名为 `dns deploy` | `dns deploy` |

---

## 常见问题

### Q: 为什么删除了 `plan` 命令？

A: `plan` 命令需要二次执行（先 plan 再 apply），用户体验不佳。新版本使用 `--dry-run` 参数，在同一命令中完成预览和执行，更简洁。

### Q: 为什么 `-e` 变为必填？

A: 禁止跨环境操作，避免误操作。所有命令均强制要求指定环境。

### Q: 为什么删除了 `server check` 和 `server sync`？

A: 这两个命令的功能被 `server setup --dry-run` 和 `server setup --yes` 完全覆盖，合并后命令结构更简洁。

### Q: `--type` 参数的语义变了？

A: 是的。旧版本中 `--biz`/`--infra` 是按服务名称筛选，新版本中 `--type biz`/`--type infra` 是按类别筛选。如需精确到具体服务，请使用 `--zone`/`--server` 缩小范围，或使用 TUI 的逐项勾选功能。

### Q: 如何在脚本中按服务名称精确部署？

A: CLI 不提供按服务名称筛选的参数。建议：
1. 使用 `--zone`/`--server` 缩小范围
2. 使用 TUI 的逐项勾选功能
3. 确保每个服务器上只部署需要更新的服务
