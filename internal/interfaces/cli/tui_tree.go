package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/litelake/yamlops/internal/application/orchestrator"
	"github.com/litelake/yamlops/internal/application/plan"
	"github.com/litelake/yamlops/internal/application/usecase"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/infrastructure/persistence"
)

func (m *Model) loadConfig() {
	if m.Config != nil {
		return
	}
	loader := persistence.NewConfigLoader(m.ConfigDir)
	cfg, err := loader.Load(nil, string(m.Environment))
	if err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Failed to load config: %v", err)
		return
	}
	if err := loader.Validate(cfg); err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Validation error: %v", err)
		return
	}
	m.Config = cfg
	m.buildTrees()
}

func (m *Model) loadConfigAsync() tea.Cmd {
	return func() tea.Msg {
		loader := persistence.NewConfigLoader(m.ConfigDir)
		cfg, err := loader.Load(nil, string(m.Environment))
		if err != nil {
			return configLoadedMsg{err: err}
		}
		if err := loader.Validate(cfg); err != nil {
			return configLoadedMsg{err: err}
		}
		return configLoadedMsg{config: cfg}
	}
}

func (m *Model) buildTrees() {
	if m.Config == nil {
		return
	}
	m.Tree.TreeNodes = m.buildAppTree()
	m.Tree.DNSTreeNodes = m.buildDNSTree()
}

func (m *Model) buildAppTree() []*TreeNode {
	if m.Config == nil {
		return nil
	}
	zoneMap := make(map[string]*TreeNode)
	serverByZone := make(map[string][]*TreeNode)
	serviceByServer := make(map[string][]*TreeNode)
	for _, z := range m.Config.Zones {
		zoneNode := &TreeNode{
			ID:       fmt.Sprintf("zone:%s", z.Name),
			Type:     NodeTypeZone,
			Name:     z.Name,
			Info:     z.Description,
			Expanded: true,
		}
		zoneMap[z.Name] = zoneNode
	}
	for _, srv := range m.Config.Servers {
		serverNode := &TreeNode{
			ID:       fmt.Sprintf("server:%s", srv.Name),
			Type:     NodeTypeServer,
			Name:     srv.Name,
			Info:     srv.IP.Public,
			Expanded: true,
		}
		if zNode, ok := zoneMap[srv.Zone]; ok {
			serverNode.Parent = zNode
			zNode.Children = append(zNode.Children, serverNode)
		}
		serverByZone[srv.Zone] = append(serverByZone[srv.Zone], serverNode)
		serviceByServer[srv.Name] = []*TreeNode{}
	}
	for _, infra := range m.Config.InfraServices {
		infraNode := &TreeNode{
			ID:   fmt.Sprintf("infra:%s", infra.Name),
			Type: NodeTypeInfra,
			Name: infra.Name,
			Info: m.getServicePortsInfo(infra.Server),
		}
		for _, sn := range serverByZone {
			for _, s := range sn {
				if s.Name == infra.Server {
					infraNode.Parent = s
					s.Children = append(s.Children, infraNode)
				}
			}
		}
	}
	for _, svc := range m.Config.Services {
		svcNode := &TreeNode{
			ID:   fmt.Sprintf("biz:%s", svc.Name),
			Type: NodeTypeBiz,
			Name: svc.Name,
			Info: m.getBizServicePortsInfo(svc),
		}
		for _, z := range m.Config.Zones {
			for _, srv := range m.Config.Servers {
				if srv.Name == svc.Server && srv.Zone == z.Name {
					if zNode, ok := zoneMap[z.Name]; ok {
						for _, sNode := range zNode.Children {
							if sNode.Name == srv.Name {
								svcNode.Parent = sNode
								sNode.Children = append(sNode.Children, svcNode)
							}
						}
					}
				}
			}
		}
	}
	var roots []*TreeNode
	for _, z := range m.Config.Zones {
		if zNode, ok := zoneMap[z.Name]; ok {
			roots = append(roots, zNode)
		}
	}
	return roots
}

func (m *Model) getServicePortsInfo(serverName string) string {
	for _, srv := range m.Config.Servers {
		if srv.Name == serverName {
			return ""
		}
	}
	return ""
}

func (m *Model) getBizServicePortsInfo(svc entity.BizService) string {
	if len(svc.Ports) == 0 {
		return ""
	}
	var ports []string
	for _, p := range svc.Ports {
		ports = append(ports, fmt.Sprintf(":%d", p.Host))
	}
	return strings.Join(ports, ",")
}

func (m *Model) buildDNSTree() []*TreeNode {
	if m.Config == nil {
		return nil
	}
	domainMap := make(map[string]*TreeNode)
	for _, d := range m.Config.Domains {
		domainNode := &TreeNode{
			ID:       fmt.Sprintf("domain:%s", d.Name),
			Type:     NodeTypeDomain,
			Name:     d.Name,
			Info:     d.DNSISP,
			Expanded: false,
		}
		domainMap[d.Name] = domainNode
		for _, r := range d.Records {
			recordNode := &TreeNode{
				ID:   fmt.Sprintf("record:%s:%s:%s", d.Name, r.Type, r.Name),
				Type: NodeTypeDNSRecord,
				Name: fmt.Sprintf("%-6s %s", r.Type, r.Name),
				Info: r.Value,
			}
			recordNode.Parent = domainNode
			domainNode.Children = append(domainNode.Children, recordNode)
		}
	}
	var roots []*TreeNode
	for _, d := range m.Config.Domains {
		if dNode, ok := domainMap[d.Name]; ok {
			roots = append(roots, dNode)
		}
	}
	return roots
}

func (m *Model) generatePlan() {
	m.Action.PlanResult = valueobject.NewPlan()
	m.UI.ErrorMessage = ""
	m.loadConfig()
	if m.UI.ErrorMessage != "" {
		return
	}
	m.buildScopeFromSelection()

	var state *plan.DeploymentState
	if m.ViewMode == ViewModeDNS {
		state = m.fetchDNSRemoteState()
	} else {
		fetcher := orchestrator.NewStateFetcher(string(m.Environment), m.ConfigDir)
		state = fetcher.Fetch(context.Background(), m.Config)
	}

	planner := plan.NewPlanner(
		plan.WithConfig(m.Config),
		plan.WithEnv(string(m.Environment)),
		plan.WithState(state),
	)
	executionPlan, err := planner.Plan(m.Action.PlanScope)
	if err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Failed to generate plan: %v", err)
		return
	}
	m.Action.PlanResult = executionPlan
	m.Action.ApplyTotal = len(executionPlan.Changes())
	if m.Action.ApplyTotal == 0 {
		m.Action.ApplyTotal = 1
	}
}

func (m *Model) generatePlanAsync() tea.Cmd {
	return func() tea.Msg {
		executionPlan := valueobject.NewPlan()
		if m.Config == nil {
			loader := persistence.NewConfigLoader(m.ConfigDir)
			cfg, err := loader.Load(nil, string(m.Environment))
			if err != nil {
				return planGeneratedMsg{err: err}
			}
			if err := loader.Validate(cfg); err != nil {
				return planGeneratedMsg{err: err}
			}
			m.Config = cfg
		}

		scope := valueobject.NewScope().WithDNSOnly(m.ViewMode == ViewModeDNS)
		services := make(map[string]bool)
		infraServices := make(map[string]bool)
		domains := make(map[string]bool)
		currentTree := m.getCurrentTree()
		for _, node := range currentTree {
			leaves := node.GetSelectedLeaves()
			for _, leaf := range leaves {
				switch leaf.Type {
				case NodeTypeInfra:
					infraServices[leaf.Name] = true
				case NodeTypeBiz:
					services[leaf.Name] = true
				case NodeTypeDNSRecord:
					parts := strings.Split(leaf.ID, ":")
					if len(parts) >= 2 {
						domains[parts[1]] = true
					}
				}
			}
		}
		var svcList []string
		for svc := range services {
			svcList = append(svcList, svc)
		}
		var infraList []string
		for infra := range infraServices {
			infraList = append(infraList, infra)
		}
		scope = scope.WithServices(svcList).WithInfraServices(infraList)
		if len(svcList) > 0 || len(infraList) > 0 {
			scope = scope.WithForceDeploy(true)
		}
		for d := range domains {
			scope = scope.WithDomain(d)
			break
		}

		var state *plan.DeploymentState
		if m.ViewMode == ViewModeDNS {
			state = m.fetchDNSRemoteState()
		} else {
			fetcher := orchestrator.NewStateFetcher(string(m.Environment), m.ConfigDir)
			state = fetcher.Fetch(context.Background(), m.Config)
		}

		planner := plan.NewPlanner(
			plan.WithConfig(m.Config),
			plan.WithEnv(string(m.Environment)),
			plan.WithState(state),
		)
		executionPlan, err := planner.Plan(scope)
		if err != nil {
			return planGeneratedMsg{err: err}
		}
		return planGeneratedMsg{plan: executionPlan}
	}
}

func (m *Model) fetchDNSRemoteState() *plan.DeploymentState {
	state := &plan.DeploymentState{
		Services:      make(map[string]*entity.BizService),
		InfraServices: make(map[string]*entity.InfraService),
		Servers:       make(map[string]*entity.Server),
		Zones:         make(map[string]*entity.Zone),
		Domains:       make(map[string]*entity.Domain),
		Records:       make(map[string]*entity.DNSRecord),
		ISPs:          make(map[string]*entity.ISP),
	}

	selectedDomains := m.getSelectedDomains()
	if len(selectedDomains) == 0 {
		return state
	}

	for _, domainName := range selectedDomains {
		domain := m.Config.GetDomainMap()[domainName]
		if domain == nil {
			continue
		}
		isp := m.Config.GetISPMap()[domain.DNSISP]
		if isp == nil {
			continue
		}
		provider, err := createDNSProviderFromConfig(isp, m.Config.GetSecretsMap())
		if err != nil {
			continue
		}
		remoteRecords, err := provider.ListRecords(context.Background(), domainName)
		if err != nil {
			continue
		}
		for _, rr := range remoteRecords {
			recordName := rr.Name
			if recordName == domainName || recordName == "" {
				recordName = "@"
			} else if strings.HasSuffix(rr.Name, "."+domainName) {
				recordName = strings.TrimSuffix(rr.Name, "."+domainName)
			}
			key := fmt.Sprintf("%s:%s:%s", domainName, rr.Type, recordName)
			state.Records[key] = &entity.DNSRecord{
				Domain: domainName,
				Type:   entity.DNSRecordType(rr.Type),
				Name:   recordName,
				Value:  rr.Value,
				TTL:    rr.TTL,
			}
		}
	}

	for _, d := range m.Config.Domains {
		state.Domains[d.Name] = &d
	}

	return state
}

func (m *Model) getSelectedDomains() []string {
	domainSet := make(map[string]bool)
	currentTree := m.getCurrentTree()
	for _, node := range currentTree {
		leaves := node.GetSelectedLeaves()
		for _, leaf := range leaves {
			if leaf.Type == NodeTypeDNSRecord {
				parts := strings.Split(leaf.ID, ":")
				if len(parts) >= 2 {
					domainSet[parts[1]] = true
				}
			}
		}
		for _, child := range node.Children {
			if child.Selected && child.Type == NodeTypeDomain {
				domainSet[child.Name] = true
			}
		}
	}
	var domains []string
	for d := range domainSet {
		domains = append(domains, d)
	}
	return domains
}

func (m *Model) buildScopeFromSelection() {
	m.Action.PlanScope = valueobject.NewScope().WithDNSOnly(m.ViewMode == ViewModeDNS)
	services := make(map[string]bool)
	infraServices := make(map[string]bool)
	domains := make(map[string]bool)
	currentTree := m.getCurrentTree()
	for _, node := range currentTree {
		leaves := node.GetSelectedLeaves()
		for _, leaf := range leaves {
			switch leaf.Type {
			case NodeTypeInfra:
				infraServices[leaf.Name] = true
			case NodeTypeBiz:
				services[leaf.Name] = true
			case NodeTypeDNSRecord:
				parts := strings.Split(leaf.ID, ":")
				if len(parts) >= 2 {
					domains[parts[1]] = true
				}
			}
		}
	}
	var svcList []string
	for svc := range services {
		svcList = append(svcList, svc)
	}
	var infraList []string
	for infra := range infraServices {
		infraList = append(infraList, infra)
	}
	m.Action.PlanScope = m.Action.PlanScope.WithServices(svcList).WithInfraServices(infraList)
	if len(svcList) > 0 || len(infraList) > 0 {
		m.Action.PlanScope = m.Action.PlanScope.WithForceDeploy(true)
	}
	for d := range domains {
		m.Action.PlanScope = m.Action.PlanScope.WithDomain(d)
		break
	}
}

func (m *Model) executeApplyAsync() tea.Cmd {
	return func() tea.Msg {
		if m.Action.PlanResult == nil || !m.Action.PlanResult.HasChanges() {
			return applyCompleteAsyncMsg{}
		}
		if m.Config == nil {
			loader := persistence.NewConfigLoader(m.ConfigDir)
			cfg, err := loader.Load(nil, string(m.Environment))
			if err != nil {
				return applyCompleteAsyncMsg{err: err}
			}
			m.Config = cfg
		}
		planner := plan.NewPlanner(
			plan.WithConfig(m.Config),
			plan.WithEnv(string(m.Environment)),
		)
		if err := planner.GenerateDeployments(); err != nil {
			return applyCompleteAsyncMsg{err: err}
		}
		executor := usecase.NewExecutor(&usecase.ExecutorConfig{
			Plan: m.Action.PlanResult,
			Env:  string(m.Environment),
		})
		executor.SetSecrets(m.Config.GetSecretsMap())
		executor.SetDomains(m.Config.GetDomainMap())
		executor.SetISPs(m.Config.GetISPMap())
		executor.SetServerEntities(m.Config.GetServerMap())
		executor.SetWorkDir(m.ConfigDir)
		secrets := m.Config.GetSecretsMap()
		for _, srv := range m.Config.Servers {
			password, err := srv.SSH.Password.Resolve(secrets)
			if err != nil {
				continue
			}
			executor.RegisterServer(srv.Name, srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
		}
		results := executor.Apply()
		return applyCompleteAsyncMsg{results: results}
	}
}

func (m Model) getCurrentTree() []*TreeNode {
	if m.ViewMode == ViewModeDNS {
		return m.Tree.DNSTreeNodes
	}
	return m.Tree.TreeNodes
}

func (m Model) countVisibleNodes() int {
	count := 0
	for _, node := range m.getCurrentTree() {
		count += len(node.GetVisibleNodes())
	}
	return count
}

func (m Model) getNodeAtIndex(index int) *TreeNode {
	count := 0
	for _, node := range m.getCurrentTree() {
		visible := node.GetVisibleNodes()
		if index < count+len(visible) {
			return visible[index-count]
		}
		count += len(visible)
	}
	return nil
}
