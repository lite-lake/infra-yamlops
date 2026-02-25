# 代码冗余汇总报告

## 1. 重复常量定义

**位置**: `internal/domain/constants.go` 与 `internal/constants/constants.go`

```go
// 完全重复的常量
const MaxPortNumber = 65535
const (
    DefaultRetryMaxAttempts    = 3
    DefaultRetryInitialDelayMs = 100
    DefaultRetryMaxDelaySec    = 30
    DefaultRetryMultiplier     = 2.0
)
```

**建议**: 统一移至 `internal/constants/` 目录，删除重复定义。

---

## 2. 重复接口定义

**位置**: `internal/domain/contract/handler.go` 与 `internal/application/handler/types.go`

- Handler
- DepsProvider
- DNSDeps
- ServiceDeps
- CommonDeps
- Result

**建议**: 统一使用 `internal/domain/contract/` 中的定义。

---

## 3. DNS 提供商重复代码

### 3.1 记录列表查询
**文件**: 
- `internal/infrastructure/dns/cloudflare.go:43-77,248-277`
- `internal/infrastructure/dns/aliyun.go:34-60,132-159`
- `internal/infrastructure/dns/tencent.go:33-59,140-167`

### 3.2 按名称查询记录
**文件**: 
- `internal/infrastructure/dns/aliyun.go:161-188`
- `internal/infrastructure/dns/tencent.go:169-196`

### 3.3 TTL 解析函数
**文件**: 
- `internal/infrastructure/dns/aliyun.go:212-218`
- `internal/infrastructure/dns/tencent.go:260-266`

```go
// 高度相似的实现，仅返回类型不同
func ParseAliyunTTL(ttlStr string) (int64, error)
func ParseTencentTTL(ttlStr string) (uint64, error)
```

### 3.4 批处理操作
**文件**: 
- `internal/infrastructure/dns/cloudflare.go:279-295`
- `internal/infrastructure/dns/aliyun.go:190-206`
- `internal/infrastructure/dns/tencent.go:198-214`

**建议**: 提取通用逻辑到 `common.go`。

---

## 4. 无意义包装函数

**位置**: `internal/infrastructure/dns/cloudflare.go:176-178`

```go
func parseSRVValue(value string) (priority, weight, port float64, target string) {
    return ParseSRVValue(value)  // 直接调用已有函数
}
```

**建议**: 直接使用 `ParseSRVValue`，删除包装函数。

---

## 5. 重复比较函数模式

**位置**: `internal/domain/service/differ_servers.go:63-431`

多个 `*Equal` 函数采用相同的 nil 检查和字段比较模式：
- gatewayPortsEqual
- gatewayConfigEqual
- gatewaySSLConfigEqual
- gatewayWAFConfigEqual
- sslConfigEqual

**建议**: 重构为通用比较辅助函数。

---

## 6. 重复接口方法

**位置**: 
- `internal/domain/entity/biz_service.go:129-135`
- `internal/domain/entity/infra_service.go:221-227`

```go
// 两个结构体实现相同方法
func (s *BizService) GetServer() string { return s.Server }
func (s *BizService) GetNetworks() []string { return s.Networks }

func (s *InfraService) GetServer() string { return s.Server }
func (s *InfraService) GetNetworks() []string { return s.Networks }
```

**建议**: 使用结构体嵌入共享通用字段和方法。

---

## 优先级排序

| 优先级 | 问题 | 影响 |
|--------|------|------|
| 高 | 重复常量 | 维护困难，易不一致 |
| 高 | 重复接口 | 架构混乱 |
| 中 | DNS 提供商重复 | 代码膨胀 |
| 中 | 重复比较函数 | 难以维护 |
| 低 | 包装函数 | 轻微冗余 |
| 低 | 重复接口方法 | 可通过嵌入解决 |
