# 合并 domains.yaml 和 dns.yaml 改造方案

## 1. 背景与目标

### 1.1 现状分析

当前系统将 DNS 相关配置分散在两个文件中：

- **domains.yaml**: 域名元信息（ISP归属、DNS服务商、父域名）
- **dns.yaml**: DNS 解析记录（A/CNAME/MX/TXT 等）

**示例**:
```yaml
# userdata/prod/domains.yaml
domains:
  - name: example.com
    isp: aliyun
    dns_isp: cloudflare

# userdata/prod/dns.yaml
records:
  - domain: example.com
    type: A
    name: "@"
    value: 203.0.113.10
    ttl: 300
```

### 1.2 问题

1. **维护分散**: 同一域名的配置在两个文件间跳转，增加认知负担
2. **重复信息**: dns.yaml 中每条记录都需重复 `domain` 字段
3. **可读性差**: 难以快速了解某域名下的完整 DNS 配置
4. **文件冗长**: dns.yaml 容易变得很长，难以定位特定域名的记录

### 1.3 目标

合并为单一 `dns.yaml`，采用嵌套结构：

```yaml
# userdata/prod/dns.yaml
domains:
  - name: example.com
    isp: aliyun
    dns_isp: cloudflare
    records:
      - type: A
        name: "@"
        value: 203.0.113.10
        ttl: 300
      - type: A
        name: www
        value: 203.0.113.10
        ttl: 300
```

---

## 2. 影响范围

### 2.1 涉及的代码文件

| 文件路径 | 变更类型 | 说明 |
|---------|---------|------|
| `internal/domain/entity/domain.go` | 修改 | 添加 `Records` 字段 |
| `internal/domain/entity/dns_record.go` | 保留 | DNSRecord 实体保持不变 |
| `internal/domain/entity/config.go` | 修改 | 移除 `DNSRecords` 字段，调整 `GetDomainMap` |
| `internal/infrastructure/persistence/config_loader.go` | 修改 | 合并加载逻辑 |
| `internal/domain/service/planner_records.go` | 修改 | 适配新数据结构 |
| `internal/plan/planner.go` | 修改 | 适配新数据结构 |
| `internal/interfaces/cli/dns.go` | 修改 | 适配新数据结构 |
| `internal/interfaces/cli/dns_pull.go` | 修改 | 保存逻辑适配 |
| `internal/interfaces/cli/list.go` | 修改 | 显示逻辑适配 |
| `internal/interfaces/cli/show.go` | 修改 | 显示逻辑适配 |
| `internal/interfaces/cli/tui_tree.go` | 修改 | 树形展示适配 |
| `internal/interfaces/cli/tui_actions.go` | 修改 | TUI 操作适配 |
| `internal/application/handler/dns_handler.go` | 无需修改 | 使用 DNSRecord 实体 |
| `internal/domain/repository/state.go` | 保留 | Records 仍独立存储在状态中 |
| `userdata/*/domains.yaml` | 删除 | 迁移后删除 |
| `userdata/*/dns.yaml` | 迁移 | 按新格式重写 |

### 2.2 保持不变的接口

- **Handler 接口**: `DNSHandler.Apply()` 仍接收 `*entity.DNSRecord`
- **State 结构**: `DeploymentState.Records` 保持 `map[string]*entity.DNSRecord`
- **Provider 接口**: DNS Provider API 不变

---

## 3. 数据结构变更

### 3.1 Domain 实体 (修改)

```go
// internal/domain/entity/domain.go

type DNSRecord struct {
    Type   DNSRecordType `yaml:"type"`
    Name   string        `yaml:"name"`
    Value  string        `yaml:"value"`
    TTL    int           `yaml:"ttl"`
}

type Domain struct {
    Name    string       `yaml:"name"`
    ISP     string       `yaml:"isp,omitempty"`
    DNSISP  string       `yaml:"dns_isp"`
    Parent  string       `yaml:"parent,omitempty"`
    Records []DNSRecord  `yaml:"records,omitempty"`  // 新增
}

func (d *Domain) Validate() error {
    // 现有验证...
    
    // 新增：验证嵌套记录
    for i, r := range d.Records {
        if err := r.Validate(); err != nil {
            return fmt.Errorf("records[%d]: %w", i, err)
        }
    }
    return nil
}

// 新增：扁平化获取所有记录（兼容现有逻辑）
func (d *Domain) FlattenRecords() []DNSRecord {
    var result []DNSRecord
    for _, r := range d.Records {
        record := r
        record.Domain = d.Name  // 填充 Domain 字段
        result = append(result, record)
    }
    return result
}
```

### 3.2 Config 实体 (修改)

```go
// internal/domain/entity/config.go

type Config struct {
    // ... 其他字段保持不变
    Domains []Domain `yaml:"domains,omitempty"`
    // DNSRecords []DNSRecord `yaml:"records,omitempty"`  // 移除
}

// GetDomainMap 保持不变
func (c *Config) GetDomainMap() map[string]*Domain {
    m := make(map[string]*Domain)
    for i := range c.Domains {
        m[c.Domains[i].Name] = &c.Domains[i]
    }
    return m
}

// 新增：获取所有扁平化的 DNS 记录
func (c *Config) GetAllDNSRecords() []DNSRecord {
    var records []DNSRecord
    for _, d := range c.Domains {
        for _, r := range d.Records {
            record := r
            record.Domain = d.Name
            records = append(records, record)
        }
    }
    return records
}
```

### 3.3 YAML 文件格式 (迁移)

**迁移前** (两个文件):

```yaml
# domains.yaml
domains:
  - name: example.com
    isp: aliyun
    dns_isp: cloudflare

# dns.yaml
records:
  - domain: example.com
    type: A
    name: "@"
    value: 203.0.113.10
```

**迁移后** (单一文件):

```yaml
# dns.yaml
domains:
  - name: example.com
    isp: aliyun
    dns_isp: cloudflare
    records:
      - type: A
        name: "@"
        value: 203.0.113.10
        ttl: 300
```

---

## 4. 实现步骤

### 4.1 Phase 1: 实体层改造

**任务边界**: 修改 Domain 和 Config 实体，保持向后兼容

#### Step 1.1: 修改 DNSRecord 实体

文件: `internal/domain/entity/dns_record.go`

1. 在 `DNSRecord` 结构体中添加 `Domain` 字段（用于内部处理，非 YAML 序列化）:

```go
type DNSRecord struct {
    Domain DNSRecordType `yaml:"-"`      // 非序列化，运行时填充
    Type   DNSRecordType `yaml:"type"`
    Name   string        `yaml:"name"`
    Value  string        `yaml:"value"`
    TTL    int           `yaml:"ttl"`
}
```

#### Step 1.2: 修改 Domain 实体

文件: `internal/domain/entity/domain.go`

1. 添加 `Records` 字段
2. 更新 `Validate()` 方法验证嵌套记录
3. 添加 `FlattenRecords()` 方法

#### Step 1.3: 修改 Config 实体

文件: `internal/domain/entity/config.go`

1. 移除 `DNSRecords` 字段
2. 添加 `GetAllDNSRecords()` 方法
3. 更新 `Validate()` 方法

### 4.2 Phase 2: 基础设施层改造

**任务边界**: 修改配置加载逻辑

#### Step 2.1: 修改配置加载器

文件: `internal/infrastructure/persistence/config_loader.go`

1. 移除 `loadDNSRecords` 函数
2. 修改 `loaders` 切片，移除 `dns.yaml` 条目
3. 更新 `validateDNSReferences` 逻辑（或移除，因为记录已嵌入域名）
4. 保留 `domains.yaml` 加载（新格式下 Records 会自动解析）

**关键代码变更**:

```go
loaders := []struct {
    filename string
    loader   func(string, *entity.Config) error
}{
    // ... 其他加载器
    {"domains.yaml", loadDomains},  // 现在加载包含 records 的 domain
    // {"dns.yaml", loadDNSRecords},  // 移除
}
```

#### Step 2.2: 验证逻辑调整

- `validateDNSReferences`: 不再需要验证记录引用的域名（已在同一结构内）
- `validateDomainConflicts`: 保留，检查记录重复

### 4.3 Phase 3: 业务逻辑层改造

**任务边界**: 修改 Planner 服务

#### Step 3.1: 修改 PlannerService

文件: `internal/domain/service/planner_records.go`

1. 更新 `PlanRecords` 方法签名:

```go
// 旧
func (s *PlannerService) PlanRecords(plan *valueobject.Plan, cfgRecords []entity.DNSRecord, scope *valueobject.Scope)

// 新
func (s *PlannerService) PlanRecords(plan *valueobject.Plan, domains map[string]*entity.Domain, scope *valueobject.Scope) {
    // 遍历 domains，扁平化记录后进行对比
}
```

#### Step 3.2: 修改 Planner

文件: `internal/plan/planner.go`

1. 更新 `Plan` 方法调用:

```go
// 旧
p.plannerService.PlanRecords(plan, p.config.DNSRecords, scope)

// 新
p.plannerService.PlanRecords(plan, p.config.GetDomainMap(), scope)
```

2. 更新 `LoadStateFromFile` 和 `SaveStateToFile` 方法中的记录处理逻辑

### 4.4 Phase 4: CLI 层改造

**任务边界**: 修改命令行界面相关代码

#### Step 4.1: 修改 dns.go

文件: `internal/interfaces/cli/dns.go`

1. `printDNSRecords`: 使用 `cfg.GetAllDNSRecords()` 获取记录
2. `runDNSShow`: 适配新的记录查找逻辑
3. `filterDNSChanges`: 保持不变（操作 Change 对象）

#### Step 4.2: 修改 dns_pull.go

文件: `internal/interfaces/cli/dns_pull.go`

1. `saveDomainDiffs` 和 `saveRecordDiffs`: 
   - 合并为单一保存逻辑
   - 保存到合并后的 `dns.yaml` 格式

**关键变更**:

```go
func saveDNSConfig(configDir string, cfg *entity.Config) error {
    dnsPath := filepath.Join(configDir, "dns.yaml")
    return saveYAMLFile(dnsPath, "domains", cfg.Domains)
}
```

#### Step 4.3: 修改 list.go 和 show.go

- `list.go`: 使用 `cfg.GetAllDNSRecords()` 替代 `cfg.DNSRecords`
- `show.go`: 保持不变（显示 Domain 实体）

#### Step 4.4: 修改 TUI 相关文件

文件: 
- `internal/interfaces/cli/tui_tree.go`
- `internal/interfaces/cli/tui_actions.go`
- `internal/interfaces/cli/tui_menu.go`
- `internal/interfaces/cli/tui_view.go`

1. 使用 `cfg.GetAllDNSRecords()` 获取记录列表
2. 保存时写入合并后的格式
3. 调整树形展示逻辑（记录已嵌入域名节点）

### 4.5 Phase 5: 数据迁移

**任务边界**: 迁移现有配置文件

#### Step 5.1: 编写迁移脚本

创建临时迁移命令或脚本:

```go
// cmd/migrate/main.go
func migrateDNSConfig(env string) error {
    // 1. 读取 domains.yaml
    // 2. 读取 dns.yaml
    // 3. 合并为新格式
    // 4. 写入新的 dns.yaml
    // 5. 备份/删除旧的 domains.yaml
}
```

#### Step 5.2: 执行迁移

```bash
# 对每个环境执行
yamlops migrate dns --env prod
yamlops migrate dns --env staging
yamlops migrate dns --env dev
```

---

## 5. 测试策略

### 5.1 单元测试

| 测试文件 | 测试内容 |
|---------|---------|
| `domain_test.go` | Domain.Validate() 包含 Records 验证 |
| `domain_test.go` | Domain.FlattenRecords() 正确填充 Domain 字段 |
| `config_test.go` | Config.GetAllDNSRecords() 正确收集所有记录 |
| `config_loader_test.go` | 加载新格式 YAML 文件 |

### 5.2 集成测试

1. **配置加载**: 确保新格式正确解析
2. **Plan 生成**: 确保变更检测正确
3. **Apply 执行**: 确保 DNS 记录正确创建/更新/删除
4. **Pull 操作**: 确保远程记录正确同步到本地

### 5.3 手动验证

```bash
# 1. 验证配置加载
yamlops validate --env prod

# 2. 验证 Plan 生成
yamlops dns plan --env prod

# 3. 验证列表显示
yamlops dns list

# 4. 验证 TUI
yamlops tui
```

---

## 6. 回滚方案

### 6.1 代码回滚

保留 Git 标签/分支记录合并前状态，可随时回滚代码。

### 6.2 数据回滚

迁移脚本应保留原始文件备份:

```
userdata/prod/domains.yaml.bak
userdata/prod/dns.yaml.bak
```

---

## 7. 注意事项

### 7.1 保持质量

1. **遵循现有代码风格**: 参考 AGENTS.md 中的规范
2. **错误处理**: 使用 `fmt.Errorf` 包装错误
3. **测试覆盖**: 新增代码需有对应测试
4. **文档更新**: 更新 AGENTS.md 中的配置文件列表

### 7.2 兼容性

1. **State 文件**: `DeploymentState.Records` 保持 `map[string]*entity.DNSRecord` 格式
2. **Handler 接口**: DNS Handler 接收扁平化的 `*entity.DNSRecord`
3. **Provider 接口**: 完全不受影响

### 7.3 风险点

1. **大文件性能**: 单域名记录过多时可能影响解析性能（通常不会）
2. **并发写入**: TUI 和 CLI 同时操作需注意（现有逻辑已处理）

---

## 8. 预期收益

1. **减少文件数量**: 每个 environment 少一个配置文件
2. **提升可维护性**: 域名相关配置集中管理
3. **减少重复**: 不再每条记录重复 domain 字段
4. **更清晰的层次**: YAML 结构更直观

---

## 9. 时间估算

| 阶段 | 预计工时 |
|-----|---------|
| Phase 1: 实体层 | 2h |
| Phase 2: 基础设施层 | 2h |
| Phase 3: 业务逻辑层 | 2h |
| Phase 4: CLI 层 | 3h |
| Phase 5: 数据迁移 | 1h |
| 测试与验证 | 2h |
| **总计** | **12h** |

---

## 10. 附录

### 10.1 完整的新格式示例

```yaml
# userdata/prod/dns.yaml
domains:
  - name: example.com
    isp: aliyun
    dns_isp: cloudflare
    records:
      - type: A
        name: "@"
        value: 203.0.113.10
        ttl: 300
      - type: A
        name: www
        value: 203.0.113.10
        ttl: 300
      - type: CNAME
        name: api
        value: api.internal.example.com
        ttl: 600
      - type: MX
        name: "@"
        value: mail.example.com
        ttl: 3600
      - type: TXT
        name: "@"
        value: "v=spf1 include:_spf.google.com ~all"
        ttl: 3600

  - name: api.example.com
    parent: example.com
    dns_isp: cloudflare
    records:
      - type: A
        name: "@"
        value: 203.0.113.20
        ttl: 300

  - name: "*.example.com"
    parent: example.com
    dns_isp: cloudflare
    # 通配符域名可能没有记录
```

### 10.2 DNSRecord 验证规则

记录类型有效值:
- `A`: IPv4 地址
- `AAAA`: IPv6 地址
- `CNAME`: 域名
- `MX`: 邮件服务器
- `TXT`: 文本
- `NS`: 名称服务器
- `SRV`: 服务记录
