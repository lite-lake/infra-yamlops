package orchestrator

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lite-lake/infra-yamlops/internal/constants"
	domainerr "github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/repository"
	"github.com/lite-lake/infra-yamlops/internal/domain/retry"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/dns"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/logger"
)

// dnsProviderFactory 抽象 DNS Provider 的创建，便于测试注入。
type dnsProviderFactory interface {
	Create(isp *entity.ISP, secrets map[string]string) (dns.Provider, error)
}

// FetchDNSRemoteState 读取选中域名的远端 DNS 记录并构造部署状态。
//
// 与 SSH 服务状态不同，DNS 远端记录**不允许失败当空**：空的 Records 会让
// differ 把配置里的每条记录都判成 create，dry-run 误报大量新增、真下发则
// 产生重复/冲突记录。因此这里对瞬时错误做有限重试，重试后仍失败即返回错误，
// 由调用方中止规划并显式暴露「远端读取失败」。
//
// 例外：没有任何配置记录的域名不会产生 create，读取失败只告警跳过，
// 避免一个已下线/未纳管的域名阻塞整批 DNS 规划（缺少删除计划是保守方向）。
func FetchDNSRemoteState(ctx context.Context, cfg *entity.Config, scope *valueobject.Scope) (*repository.DeploymentState, error) {
	return fetchDNSRemoteStateWithFactory(ctx, cfg, scope, dns.NewFactory())
}

func fetchDNSRemoteStateWithFactory(ctx context.Context, cfg *entity.Config, scope *valueobject.Scope, factory dnsProviderFactory) (*repository.DeploymentState, error) {
	state := &repository.DeploymentState{
		Services:      make(map[string]*entity.BizService),
		InfraServices: make(map[string]*entity.InfraService),
		Servers:       make(map[string]*entity.Server),
		Zones:         make(map[string]*entity.Zone),
		Domains:       make(map[string]*entity.Domain),
		Records:       make(map[string]*entity.DNSRecord),
		ISPs:          make(map[string]*entity.ISP),
	}

	selectedDomains := make(map[string]bool)
	if scope != nil && len(scope.Domains()) > 0 {
		for _, d := range scope.Domains() {
			selectedDomains[d] = true
		}
	}

	if len(selectedDomains) == 0 {
		for _, d := range cfg.Domains {
			selectedDomains[d.Name] = true
		}
	}

	// 排序保证出错时报的是稳定域名，且多域名场景下的日志顺序可复现。
	domainNames := make([]string, 0, len(selectedDomains))
	for domainName := range selectedDomains {
		domainNames = append(domainNames, domainName)
	}
	sort.Strings(domainNames)

	dnsFactory := factory
	secretsMap := cfg.GetSecretsMap()
	domainMap := cfg.GetDomainMap()
	ispMap := cfg.GetISPMap()

	for _, domainName := range domainNames {
		domain := domainMap[domainName]
		if domain == nil {
			return nil, fmt.Errorf("%w: %s", domainerr.ErrDNSDomainNotFound, domainName)
		}
		// 无配置记录的域名不会产生 create 误报；远端读取失败时仅告警跳过，
		// 避免一个已下线/未纳管的域名阻塞整批 DNS 规划（缺失删除计划是保守方向）。
		if len(domain.FlattenRecords()) == 0 {
			logger.Warn("skip fetching remote DNS records: no configured records", "domain", domainName)
			continue
		}
		isp := ispMap[domain.DNSISP]
		if isp == nil {
			return nil, fmt.Errorf("%w: %s (referenced by domain %s)", domainerr.ErrISPNotFound, domain.DNSISP, domainName)
		}
		provider, err := dnsFactory.Create(isp, secretsMap)
		if err != nil {
			return nil, fmt.Errorf("create DNS provider for ISP %s (domain %s): %w", domain.DNSISP, domainName, err)
		}
		remoteRecords, err := listDNSRecordsWithRetry(ctx, provider, domainName)
		if err != nil {
			return nil, fmt.Errorf("list remote records for domain %s (ISP %s): %w", domainName, domain.DNSISP, err)
		}
		for _, rr := range remoteRecords {
			recordName := rr.Name
			if recordName == domainName || recordName == "" {
				recordName = "@"
			} else if strings.HasSuffix(rr.Name, "."+domainName) {
				recordName = strings.TrimSuffix(rr.Name, "."+domainName)
			}
			record := &entity.DNSRecord{
				Domain: domainName,
				Type:   entity.DNSRecordType(rr.Type),
				Name:   recordName,
				Value:  rr.Value,
				TTL:    rr.TTL,
				Line:   rr.Line,
			}
			key := fmt.Sprintf("%s:%s:%s:%s:%s", domainName, rr.Type, recordName, record.EffectiveLine(), record.Value)
			state.Records[key] = record
		}
	}

	for _, d := range cfg.Domains {
		state.Domains[d.Name] = &d
	}

	return state, nil
}

// listDNSRecordsWithRetry 对瞬时失败的记录列举做有限重试，最终失败原样返回。
func listDNSRecordsWithRetry(ctx context.Context, provider dns.Provider, domainName string) ([]dns.DNSRecord, error) {
	return retry.DoWithResult(ctx, func() ([]dns.DNSRecord, error) {
		return provider.ListRecords(ctx, domainName)
	},
		retry.WithMaxAttempts(constants.DefaultDNSRetryAttempts),
		retry.WithInitialDelay(constants.DefaultDNSRetryInitialDelay),
		retry.WithMaxDelay(constants.DefaultDNSRetryMaxDelay),
		retry.WithIsRetryable(dns.IsRetryableDNSError),
		retry.WithOnRetry(func(attempt int, delay time.Duration, err error) {
			logger.Warn("retry listing remote DNS records",
				"domain", domainName,
				"attempt", attempt,
				"delay", delay,
				"error", err,
			)
		}),
	)
}
