package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/infrastructure/persistence"
	"github.com/litelake/yamlops/internal/providers/dns"
)

func newDNSPullCommand(ctx *Context) *cobra.Command {
	var (
		pullISP         string
		pullDomain      string
		pullAutoApprove bool
	)

	dnsPullCmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull DNS resources from providers",
		Long:  "Pull domains and DNS records from remote providers to local configuration.",
	}

	dnsPullDomainsCmd := &cobra.Command{
		Use:   "domains",
		Short: "Pull domains from ISP",
		Long:  "Pull domain list from specified ISP and compare with local configuration.",
		Run: func(cmd *cobra.Command, args []string) {
			runDNSPullDomains(ctx, pullISP, pullAutoApprove)
		},
	}

	dnsPullRecordsCmd := &cobra.Command{
		Use:   "records",
		Short: "Pull DNS records from domain",
		Long:  "Pull DNS records from specified domain and compare with local configuration.",
		Run: func(cmd *cobra.Command, args []string) {
			runDNSPullRecords(ctx, pullDomain, pullAutoApprove)
		},
	}

	dnsPullDomainsCmd.Flags().StringVarP(&pullISP, "isp", "i", "", "ISP name (e.g., aliyun, cloudflare, tencent)")
	dnsPullDomainsCmd.Flags().BoolVar(&pullAutoApprove, "auto-approve", false, "Auto approve all changes")

	dnsPullRecordsCmd.Flags().StringVarP(&pullDomain, "domain", "d", "", "Domain name to pull records from")
	dnsPullRecordsCmd.Flags().BoolVar(&pullAutoApprove, "auto-approve", false, "Auto approve all changes")

	dnsPullCmd.AddCommand(dnsPullDomainsCmd)
	dnsPullCmd.AddCommand(dnsPullRecordsCmd)

	return dnsPullCmd
}

type DomainDiff struct {
	Name       string
	ISP        string
	DNSISP     string
	Parent     string
	ChangeType valueobject.ChangeType
}

type RecordDiff struct {
	Domain     string
	DNSISP     string
	Type       entity.DNSRecordType
	Name       string
	Value      string
	TTL        int
	ChangeType valueobject.ChangeType
}

func runDNSPullDomains(ctx *Context, ispName string, autoApprove bool) {
	loader := persistence.NewConfigLoader(ctx.ConfigDir)
	cfg, err := loader.Load(nil, ctx.Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if ispName == "" {
		fmt.Println("Available ISPs with DNS service:")
		for _, isp := range cfg.ISPs {
			if isp.HasService(entity.ISPServiceDNS) {
				fmt.Printf("  - %s\n", isp.Name)
			}
		}
		fmt.Println("\nUsage: yamlops dns pull domains --isp <isp_name>")
		return
	}

	isp := cfg.GetISPMap()[ispName]
	if isp == nil {
		fmt.Fprintf(os.Stderr, "ISP '%s' not found\n", ispName)
		os.Exit(1)
	}
	if !isp.HasService(entity.ISPServiceDNS) {
		fmt.Fprintf(os.Stderr, "ISP '%s' does not have DNS service\n", ispName)
		os.Exit(1)
	}

	provider, err := createDNSProvider(isp, cfg.GetSecretsMap())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating DNS provider: %v\n", err)
		os.Exit(1)
	}

	remoteDomains, err := provider.ListDomains()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing domains from %s: %v\n", ispName, err)
		os.Exit(1)
	}

	localDomainMap := make(map[string]*entity.Domain)
	for i := range cfg.Domains {
		localDomainMap[cfg.Domains[i].Name] = &cfg.Domains[i]
	}

	var diffs []DomainDiff
	for _, domainName := range remoteDomains {
		if _, exists := localDomainMap[domainName]; !exists {
			diffs = append(diffs, DomainDiff{
				Name:       domainName,
				DNSISP:     ispName,
				ChangeType: valueobject.ChangeTypeCreate,
			})
		} else {
			delete(localDomainMap, domainName)
		}
	}

	for _, localDomain := range localDomainMap {
		if localDomain.DNSISP == ispName {
			diffs = append(diffs, DomainDiff{
				Name:       localDomain.Name,
				DNSISP:     localDomain.DNSISP,
				ISP:        localDomain.ISP,
				Parent:     localDomain.Parent,
				ChangeType: valueobject.ChangeTypeDelete,
			})
		}
	}

	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Name < diffs[j].Name
	})

	if len(diffs) == 0 {
		fmt.Println("No domain differences detected.")
		return
	}

	fmt.Printf("Domain Differences (ISP: %s):\n", ispName)
	fmt.Println("=================================")
	for _, diff := range diffs {
		var prefix string
		var style lipgloss.Style
		switch diff.ChangeType {
		case valueobject.ChangeTypeCreate:
			prefix = "+"
			style = changeCreateStyle
		case valueobject.ChangeTypeDelete:
			prefix = "-"
			style = changeDeleteStyle
		}
		fmt.Printf("%s %s\n", style.Render(prefix), style.Render(diff.Name))
	}

	if autoApprove {
		if err := saveDomainDiffs(ctx, diffs, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving domains: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Domains synced to local configuration.")
		return
	}

	if err := runDomainPullTUI(ctx, diffs, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runDNSPullRecords(ctx *Context, domainName string, autoApprove bool) {
	loader := persistence.NewConfigLoader(ctx.ConfigDir)
	cfg, err := loader.Load(nil, ctx.Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if domainName == "" {
		fmt.Println("Available domains:")
		for _, d := range cfg.Domains {
			fmt.Printf("  - %s (dns_isp: %s)\n", d.Name, d.DNSISP)
		}
		fmt.Println("\nUsage: yamlops dns pull records --domain <domain_name>")
		return
	}

	domain := cfg.GetDomainMap()[domainName]
	if domain == nil {
		fmt.Fprintf(os.Stderr, "Domain '%s' not found in local configuration\n", domainName)
		os.Exit(1)
	}

	isp := cfg.GetISPMap()[domain.DNSISP]
	if isp == nil {
		fmt.Fprintf(os.Stderr, "DNS ISP '%s' not found\n", domain.DNSISP)
		os.Exit(1)
	}

	provider, err := createDNSProvider(isp, cfg.GetSecretsMap())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating DNS provider: %v\n", err)
		os.Exit(1)
	}

	remoteRecords, err := provider.ListRecords(domainName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing records from %s: %v\n", domain.DNSISP, err)
		os.Exit(1)
	}

	localRecordMap := make(map[string]*entity.DNSRecord)
	for _, d := range cfg.Domains {
		for i := range d.Records {
			key := fmt.Sprintf("%s:%s", d.Records[i].Type, d.Records[i].Name)
			localRecordMap[key] = &d.Records[i]
			localRecordMap[key].Domain = d.Name
		}
	}

	var diffs []RecordDiff
	for _, remote := range remoteRecords {
		recordName := remote.Name
		if recordName == domainName || recordName == "" {
			recordName = "@"
		} else if strings.HasSuffix(remote.Name, "."+domainName) {
			recordName = strings.TrimSuffix(remote.Name, "."+domainName)
		}

		key := fmt.Sprintf("%s:%s", remote.Type, recordName)
		if local, exists := localRecordMap[key]; exists {
			if local.Value != remote.Value || local.TTL != remote.TTL {
				diffs = append(diffs, RecordDiff{
					Domain:     domainName,
					DNSISP:     domain.DNSISP,
					Type:       entity.DNSRecordType(remote.Type),
					Name:       recordName,
					Value:      remote.Value,
					TTL:        remote.TTL,
					ChangeType: valueobject.ChangeTypeUpdate,
				})
			}
			delete(localRecordMap, key)
		} else {
			diffs = append(diffs, RecordDiff{
				Domain:     domainName,
				DNSISP:     domain.DNSISP,
				Type:       entity.DNSRecordType(remote.Type),
				Name:       recordName,
				Value:      remote.Value,
				TTL:        remote.TTL,
				ChangeType: valueobject.ChangeTypeCreate,
			})
		}
	}

	for _, local := range localRecordMap {
		diffs = append(diffs, RecordDiff{
			Domain:     local.Domain,
			DNSISP:     domain.DNSISP,
			Type:       local.Type,
			Name:       local.Name,
			Value:      local.Value,
			TTL:        local.TTL,
			ChangeType: valueobject.ChangeTypeDelete,
		})
	}

	sort.Slice(diffs, func(i, j int) bool {
		if diffs[i].Name != diffs[j].Name {
			return diffs[i].Name < diffs[j].Name
		}
		return diffs[i].Type < diffs[j].Type
	})

	if len(diffs) == 0 {
		fmt.Println("No DNS record differences detected.")
		return
	}

	fmt.Printf("DNS Record Differences (Domain: %s):\n", domainName)
	fmt.Println("=====================================")
	for _, diff := range diffs {
		var prefix string
		var style lipgloss.Style
		switch diff.ChangeType {
		case valueobject.ChangeTypeCreate:
			prefix = "+"
			style = changeCreateStyle
		case valueobject.ChangeTypeUpdate:
			prefix = "~"
			style = changeUpdateStyle
		case valueobject.ChangeTypeDelete:
			prefix = "-"
			style = changeDeleteStyle
		}
		fmt.Printf("%s %-6s %-20s -> %-30s (ttl: %d)\n",
			style.Render(prefix),
			style.Render(string(diff.Type)),
			style.Render(diff.Name),
			style.Render(diff.Value),
			diff.TTL)
	}

	if autoApprove {
		if err := saveRecordDiffs(ctx, diffs, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving records: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("DNS records synced to local configuration.")
		return
	}

	if err := runRecordPullTUI(ctx, diffs, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func createDNSProvider(isp *entity.ISP, secrets map[string]string) (dns.Provider, error) {
	switch isp.Type {
	case entity.ISPTypeAliyun:
		accessKeyIDRef := isp.Credentials["access_key_id"]
		accessKeyID, err := (&accessKeyIDRef).Resolve(secrets)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve access_key_id: %w", err)
		}
		accessKeySecretRef := isp.Credentials["access_key_secret"]
		accessKeySecret, err := (&accessKeySecretRef).Resolve(secrets)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve access_key_secret: %w", err)
		}
		return dns.NewAliyunProvider(accessKeyID, accessKeySecret), nil
	case entity.ISPTypeCloudflare:
		apiTokenRef := isp.Credentials["api_token"]
		apiToken, err := (&apiTokenRef).Resolve(secrets)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve api_token: %w", err)
		}
		accountID := ""
		if accountIDRef, ok := isp.Credentials["account_id"]; ok {
			accountID, err = (&accountIDRef).Resolve(secrets)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve account_id: %w", err)
			}
		}
		return dns.NewCloudflareProvider(apiToken, accountID), nil
	case entity.ISPTypeTencent:
		secretIDRef := isp.Credentials["secret_id"]
		secretID, err := (&secretIDRef).Resolve(secrets)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve secret_id: %w", err)
		}
		secretKeyRef := isp.Credentials["secret_key"]
		secretKey, err := (&secretKeyRef).Resolve(secrets)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve secret_key: %w", err)
		}
		return dns.NewTencentProvider(secretID, secretKey), nil
	default:
		return nil, fmt.Errorf("unsupported DNS provider type: %s (ISP name: %s)", isp.Type, isp.Name)
	}
}

func saveDomainDiffs(ctx *Context, diffs []DomainDiff, cfg *entity.Config) error {
	configDir := filepath.Join(ctx.ConfigDir, "userdata", ctx.Env)
	dnsPath := filepath.Join(configDir, "dns.yaml")

	newDomains := make([]entity.Domain, 0)
	domainSet := make(map[string]bool)

	for _, diff := range diffs {
		if diff.ChangeType == valueobject.ChangeTypeCreate {
			newDomains = append(newDomains, entity.Domain{
				Name:   diff.Name,
				DNSISP: diff.DNSISP,
			})
			domainSet[diff.Name] = true
		}
	}

	for _, d := range cfg.Domains {
		if !domainSet[d.Name] {
			shouldKeep := true
			for _, diff := range diffs {
				if diff.Name == d.Name && diff.ChangeType == valueobject.ChangeTypeDelete {
					shouldKeep = false
					break
				}
			}
			if shouldKeep {
				newDomains = append(newDomains, d)
			}
		}
	}

	return saveYAMLFile(dnsPath, "domains", newDomains)
}

func saveRecordDiffs(ctx *Context, diffs []RecordDiff, cfg *entity.Config) error {
	configDir := filepath.Join(ctx.ConfigDir, "userdata", ctx.Env)
	dnsPath := filepath.Join(configDir, "dns.yaml")

	newDomains := make([]entity.Domain, 0)
	domainSet := make(map[string]bool)

	for _, diff := range diffs {
		domainSet[diff.Domain] = true
	}

	for _, d := range cfg.Domains {
		newDomain := entity.Domain{
			Name:   d.Name,
			ISP:    d.ISP,
			DNSISP: d.DNSISP,
			Parent: d.Parent,
		}
		for _, r := range d.Records {
			shouldKeep := true
			for _, diff := range diffs {
				if diff.Domain == d.Name && string(diff.Type) == string(r.Type) && diff.Name == r.Name && diff.ChangeType == valueobject.ChangeTypeDelete {
					shouldKeep = false
					break
				}
			}
			if shouldKeep {
				newDomain.Records = append(newDomain.Records, r)
			}
		}
		if domainSet[d.Name] {
			for _, diff := range diffs {
				if diff.Domain == d.Name && (diff.ChangeType == valueobject.ChangeTypeCreate || diff.ChangeType == valueobject.ChangeTypeUpdate) {
					newDomain.Records = append(newDomain.Records, entity.DNSRecord{
						Type:  diff.Type,
						Name:  diff.Name,
						Value: diff.Value,
						TTL:   diff.TTL,
					})
				}
			}
			delete(domainSet, d.Name)
		}
		newDomains = append(newDomains, newDomain)
	}

	for domainName := range domainSet {
		newDomain := entity.Domain{
			Name:    domainName,
			DNSISP:  diffs[0].DNSISP,
			Records: []entity.DNSRecord{},
		}
		for _, diff := range diffs {
			if diff.Domain == domainName && (diff.ChangeType == valueobject.ChangeTypeCreate || diff.ChangeType == valueobject.ChangeTypeUpdate) {
				newDomain.Records = append(newDomain.Records, entity.DNSRecord{
					Type:  diff.Type,
					Name:  diff.Name,
					Value: diff.Value,
					TTL:   diff.TTL,
				})
			}
		}
		newDomains = append(newDomains, newDomain)
	}

	return saveYAMLFile(dnsPath, "domains", newDomains)
}

func saveYAMLFile(path, key string, data interface{}) error {
	yamlData := map[string]interface{}{key: data}
	content, err := yaml.Marshal(yamlData)
	if err != nil {
		return fmt.Errorf("failed to marshal yaml: %w", err)
	}

	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

type PullModel struct {
	Diffs       []DomainDiff
	RecordDiffs []RecordDiff
	Selected    map[int]bool
	Cursor      int
	Width       int
	Height      int
	Mode        string
	Done        bool
	Saved       bool
	Config      *entity.Config
	IsRecords   bool
}

func (m PullModel) Init() tea.Cmd {
	return nil
}

func (m PullModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
			return m, nil
		case "down", "j":
			maxIndex := len(m.Diffs) - 1
			if m.IsRecords {
				maxIndex = len(m.RecordDiffs) - 1
			}
			if m.Cursor < maxIndex {
				m.Cursor++
			}
			return m, nil
		case " ":
			m.Selected[m.Cursor] = !m.Selected[m.Cursor]
			return m, nil
		case "a":
			for i := range m.Selected {
				m.Selected[i] = true
			}
			return m, nil
		case "n":
			for i := range m.Selected {
				m.Selected[i] = false
			}
			return m, nil
		case "enter":
			m.Done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m PullModel) View() string {
	if m.Done {
		return ""
	}

	var b strings.Builder

	title := "Select Domains to Sync"
	if m.IsRecords {
		title = "Select DNS Records to Sync"
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	if m.IsRecords {
		for i, diff := range m.RecordDiffs {
			cursor := " "
			if m.Cursor == i {
				cursor = ">"
			}
			checked := " "
			if m.Selected[i] {
				checked = "x"
			}

			var prefix string
			var style lipgloss.Style
			switch diff.ChangeType {
			case valueobject.ChangeTypeCreate:
				prefix = "+"
				style = changeCreateStyle
			case valueobject.ChangeTypeUpdate:
				prefix = "~"
				style = changeUpdateStyle
			case valueobject.ChangeTypeDelete:
				prefix = "-"
				style = changeDeleteStyle
			}

			line := fmt.Sprintf("%s [%s] %s %-6s %-20s -> %-30s",
				cursor, checked, prefix, diff.Type, diff.Name, diff.Value)
			b.WriteString(style.Render(line))
			b.WriteString("\n")
		}
	} else {
		for i, diff := range m.Diffs {
			cursor := " "
			if m.Cursor == i {
				cursor = ">"
			}
			checked := " "
			if m.Selected[i] {
				checked = "x"
			}

			var prefix string
			var style lipgloss.Style
			switch diff.ChangeType {
			case valueobject.ChangeTypeCreate:
				prefix = "+"
				style = changeCreateStyle
			case valueobject.ChangeTypeDelete:
				prefix = "-"
				style = changeDeleteStyle
			}

			line := fmt.Sprintf("%s [%s] %s %s", cursor, checked, prefix, diff.Name)
			b.WriteString(style.Render(line))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/k: up  ↓/j: down  space: toggle  a: select all  n: deselect all  enter: confirm  q: quit"))

	return b.String()
}

func runDomainPullTUI(ctx *Context, diffs []DomainDiff, cfg *entity.Config) error {
	selected := make(map[int]bool)
	for i := range diffs {
		if diffs[i].ChangeType == valueobject.ChangeTypeCreate {
			selected[i] = true
		}
	}

	m := PullModel{
		Diffs:    diffs,
		Selected: selected,
		Config:   cfg,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return err
	}

	finalModel := result.(PullModel)
	if finalModel.Done {
		selectedDiffs := make([]DomainDiff, 0)
		for i, diff := range diffs {
			if finalModel.Selected[i] {
				selectedDiffs = append(selectedDiffs, diff)
			}
		}

		if len(selectedDiffs) > 0 {
			if err := saveDomainDiffs(ctx, selectedDiffs, cfg); err != nil {
				return err
			}
			fmt.Println("Domains synced to local configuration.")
		} else {
			fmt.Println("No changes selected.")
		}
	}

	return nil
}

func runRecordPullTUI(ctx *Context, diffs []RecordDiff, cfg *entity.Config) error {
	selected := make(map[int]bool)
	for i := range diffs {
		if diffs[i].ChangeType == valueobject.ChangeTypeCreate || diffs[i].ChangeType == valueobject.ChangeTypeUpdate {
			selected[i] = true
		}
	}

	m := PullModel{
		RecordDiffs: diffs,
		Selected:    selected,
		Config:      cfg,
		IsRecords:   true,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return err
	}

	finalModel := result.(PullModel)
	if finalModel.Done {
		selectedDiffs := make([]RecordDiff, 0)
		for i, diff := range diffs {
			if finalModel.Selected[i] {
				selectedDiffs = append(selectedDiffs, diff)
			}
		}

		if len(selectedDiffs) > 0 {
			if err := saveRecordDiffs(ctx, selectedDiffs, cfg); err != nil {
				return err
			}
			fmt.Println("DNS records synced to local configuration.")
		} else {
			fmt.Println("No changes selected.")
		}
	}

	return nil
}
