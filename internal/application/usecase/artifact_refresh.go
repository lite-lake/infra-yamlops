package usecase

import (
	"path/filepath"

	"github.com/lite-lake/infra-yamlops/internal/application/generator"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

// RefreshServiceArtifacts 在下发前按当前全量配置重渲染该服务的部署产物，刷新磁盘
// deployments/ 缓存（内部PRD）。
//
// 为什么下发前必须重渲染：下发链路（deployGatewayType/SyncInfraFiles/DeployComposeFile
// 等）读取的磁盘缓存没有任何新鲜度保证——旧版本二进制的 dry-run 会把缓存覆写为缺段形态
// （如不认识 tracker_shipper 字段的版本解析时静默丢弃），不同 CWD/并发的其他 yamlops
// 进程也会写同一份缓存。缓存一旦陈旧，按缓存下发就会把陈旧字节同步到服务器并重建容器，
// 新增段静默丢失。因此产物永远从实体生成，磁盘缓存只作 diff 参照不作下发源。
//
// deps.Config() 为 nil 时直接返回 nil（兼容旧调用方/单测：未注入 config 则无重渲染
// 能力，下发退回按缓存读取的既有行为）。
func RefreshServiceArtifacts(deps DepsProvider, serviceName string) error {
	cfg := deps.Config()
	if cfg == nil {
		return nil
	}

	// 构造只含该服务的 scope：generator 的 scope 生成按 biz 列表/infra 列表分别过滤
	//（ShouldGenerateBizService/ShouldGenerateInfraService），gateway 名称不会命中任何
	// biz 服务，反之亦然，不会误刷无关产物；三遍 scoped 生成顺序与全量一致，最终落盘
	// 内容与 plan 阶段全量生成逐字节一致。
	scope := valueobject.NewScope().
		WithBizServices([]string{serviceName}).
		WithInfraServices([]string{serviceName})
	gen := generator.NewGenerator(deps.Env(), filepath.Join(deps.WorkDir(), "deployments"))
	return gen.GenerateWithScope(cfg, scope)
}
