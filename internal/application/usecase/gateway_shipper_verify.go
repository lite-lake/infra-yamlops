package usecase

// 网关服务部署后不变式校验（内部PRD §6.4，D5 编译内嵌）：
//
//	「网关服务（type=gateway 的 infra service）部署成功后，目标机渲染产物必须含
//	 tracker_shipper 段，且 enabled 值与本次部署源实体配置一致」。
//
// 该不变式是无条件总断言、独立于 checkout 新鲜度：陈旧 checkout 的实体本身就是 nil
// （services_infra.yaml 缺 shipper 段），其渲染产物同样缺段——若「双方一致即 PASS」
// 则事故原样复发。因此产物缺段一律 FAIL，不区分实体是否配置；唯一合法豁免形态是
// 实体显式声明 tracker_shipper.enabled: false（生成器对其产出 enabled:false 段），
// 实体未配置（nil）时期望 enabled=false。
//
// 渲染产物远端路径：/data/yamlops/yo-{env}-{服务名}/gateway.yml
// （constants.RemoteBaseDir + constants.ServiceNameFormat；compose 以
// ./gateway.yml:/app/configs/server.yml:ro 挂载，读宿主机文件即容器真相）。
// 校验只读（sudo cat），失败判部署失败（不自动回滚）。

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lite-lake/infra-yamlops/internal/constants"
	"github.com/lite-lake/infra-yamlops/internal/domain/contract"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/ssh"
	"gopkg.in/yaml.v3"
)

// 校验失败 sentinel（errors.Is 判别）。文案即用户可见的 FAIL 原因。
var (
	// ErrGatewayShipperArtifactUnreadable 远端 gateway.yml 读取或 YAML 解析失败
	ErrGatewayShipperArtifactUnreadable = errors.New("gateway.yml 读取/解析失败")
	// ErrGatewayShipperSegmentMissing 产物缺 tracker_shipper 段（无条件 FAIL，不区分实体
	// 是否配置——实体 nil + 产物缺段正是陈旧 checkout 的事故形态）
	ErrGatewayShipperSegmentMissing = errors.New("缺 tracker_shipper 段（dev/prod 网关必须显式配置该段；若部署源未配置则 checkout 疑似陈旧）")
	// ErrGatewayShipperEnabledMismatch 产物 enabled 与部署源实体不一致
	ErrGatewayShipperEnabledMismatch = errors.New("tracker_shipper enabled 不一致")
)

// gatewayShipperProbe 远端 gateway.yml 的最小探测结构：只关心顶层 tracker_shipper 段的
// enabled 值，其余字段（server/hosts/waf/...）全部忽略。Enabled 用 *bool 区分「键缺失」
// （生成器恒显式输出 enabled，键缺失按 false 语义处理）。
type gatewayShipperProbe struct {
	TrackerShipper *gatewayShipperEnabledProbe `yaml:"tracker_shipper"`
}

type gatewayShipperEnabledProbe struct {
	Enabled *bool `yaml:"enabled"`
}

// GatewayShipperCheck 单节点（单网关服务）校验结论。
type GatewayShipperCheck struct {
	// ServiceName 网关服务名（如 gateway-node1）
	ServiceName string
	// ServerName 目标机（节点）名，FAIL 文案中必含
	ServerName string
	// Pass true = 一致通过
	Pass bool
	// Err FAIL 时的原因（sentinel 包装，含节点名）；Pass=true 时为 nil
	Err error
}

// GatewayShipperVerifier 网关 tracker_shipper 渲染产物不变式校验器（只读）。
// SSH 执行通道为 contract.SSHRunner 窄依赖（*ssh.Client 天然实现，单测可注入 mock）。
type GatewayShipperVerifier struct {
	env string
}

func NewGatewayShipperVerifier(env string) *GatewayShipperVerifier {
	return &GatewayShipperVerifier{env: env}
}

// remoteGatewayConfigPath 远端渲染产物路径：/data/yamlops/yo-{env}-{服务名}/gateway.yml。
func (v *GatewayShipperVerifier) remoteGatewayConfigPath(serviceName string) string {
	return fmt.Sprintf("%s/%s/gateway.yml", constants.RemoteBaseDir,
		fmt.Sprintf(constants.ServiceNameFormat, v.env, serviceName))
}

// VerifyNode SSH 只读拉取单节点远端 gateway.yml 并校验 tracker_shipper 不变式。
// 期望值来自部署源实体：
//   - 产物缺 tracker_shipper 段 → 无条件 FAIL（陈旧 checkout 的实体即 nil、产物同样缺段，
//     双方「一致」不构成豁免；唯一合法豁免是显式 enabled:false）；
//   - GatewayTrackerShipper == nil（未配置）→ 期望 enabled=false（显式 enabled:false 豁免形态
//     下实体为 nil 亦可，或显式配置 enabled:false）；
//   - 段在位时 enabled 必须与实体期望一致（实体 nil 期望 false；实体未配置而产物
//     enabled:true 属不一致 FAIL）。
func (v *GatewayShipperVerifier) VerifyNode(runner contract.SSHRunner, serverName string, infra *entity.InfraService) GatewayShipperCheck {
	check := GatewayShipperCheck{ServiceName: infra.Name, ServerName: serverName}

	remotePath := v.remoteGatewayConfigPath(infra.Name)
	cmd := fmt.Sprintf("sudo cat %s", ssh.ShellEscape(remotePath))
	stdout, stderr, err := runner.Run(cmd)
	if err != nil {
		check.Err = fmt.Errorf("节点 %s 服务 %s: %w: %v (stderr: %s), path=%s",
			serverName, infra.Name, ErrGatewayShipperArtifactUnreadable, err, stderr, remotePath)
		return check
	}

	var probe gatewayShipperProbe
	if err := yaml.Unmarshal([]byte(stdout), &probe); err != nil {
		check.Err = fmt.Errorf("节点 %s 服务 %s: %w: yaml 解析失败: %v",
			serverName, infra.Name, ErrGatewayShipperArtifactUnreadable, err)
		return check
	}

	expectedEnabled := infra.GatewayTrackerShipper != nil && infra.GatewayTrackerShipper.Enabled

	if probe.TrackerShipper == nil {
		// 缺段一律 FAIL：dev/prod 网关必须显式配置该段；实体未配置而产物缺段
		// 恰是陈旧 checkout 的事故形态，不因「双方一致」放行。
		check.Err = fmt.Errorf("节点 %s 服务 %s: %w", serverName, infra.Name, ErrGatewayShipperSegmentMissing)
		return check
	}

	productEnabled := probe.TrackerShipper.Enabled != nil && *probe.TrackerShipper.Enabled
	if productEnabled != expectedEnabled {
		check.Err = fmt.Errorf("节点 %s 服务 %s: %w: 产物 %v vs 部署源 %v",
			serverName, infra.Name, ErrGatewayShipperEnabledMismatch, productEnabled, expectedEnabled)
		return check
	}

	check.Pass = true
	return check
}

// AggregateGatewayShipperChecks 汇总多节点校验结果：任一 FAIL 返回多行聚合 error
// （每个 FAIL 节点一行，含节点名与原因）；全部 PASS 返回 nil。
func AggregateGatewayShipperChecks(checks []GatewayShipperCheck) error {
	var lines []string
	for _, c := range checks {
		if c.Pass {
			continue
		}
		if c.Err != nil {
			lines = append(lines, c.Err.Error())
			continue
		}
		lines = append(lines, fmt.Sprintf("节点 %s 服务 %s: 校验失败（无原因信息）", c.ServerName, c.ServiceName))
	}
	if len(lines) == 0 {
		return nil
	}
	return fmt.Errorf("%d 台网关产物校验失败:\n%s", len(lines), strings.Join(lines, "\n"))
}
