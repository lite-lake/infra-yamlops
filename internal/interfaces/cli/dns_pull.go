package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	infradns "github.com/lite-lake/infra-yamlops/internal/infrastructure/dns"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/persistence"
	"github.com/lite-lake/infra-yamlops/internal/interfaces/shared/styles"
)

func newDNSPullCommand(ctx *Context) *cobra.Command {
	var (
		pullISP     string
		pullDomains string
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
			yes, _ := cmd.Flags().GetBool("yes")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			force, _ := cmd.Flags().GetBool("force")
			runDNSPullDomains(ctx, pullISP, yes, dryRun, force)
		},
	}

	dnsPullRecordsCmd := &cobra.Command{
		Use:   "records",
		Short: "Pull DNS records from domain",
		Long:  "Pull DNS records from specified domain and compare with local configuration.",
		Run: func(cmd *cobra.Command, args []string) {
			yes, _ := cmd.Flags().GetBool("yes")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			force, _ := cmd.Flags().GetBool("force")
			isp, _ := cmd.Flags().GetString("isp")
			runDNSPullRecords(ctx, pullDomains, yes, dryRun, force, isp)
		},
	}

	dnsPullDomainsCmd.Flags().StringVarP(&pullISP, "isp", "i", "", "ISP name (e.g., aliyun, cloudflare, tencent)")
	dnsPullDomainsCmd.Flags().BoolP("yes", "y", false, "Skip confirmation, execute all changes")
	dnsPullDomainsCmd.Flags().Bool("dry-run", false, "Preview changes without executing")
	dnsPullDomainsCmd.Flags().Bool("force", false, "Force overwrite local data even if it already exists")
	dnsPullDomainsCmd.MarkFlagRequired("isp")

	dnsPullRecordsCmd.Flags().StringVarP(&pullDomains, "domain", "d", "", "Domain name(s) to pull records from (comma-separated)")
	dnsPullRecordsCmd.Flags().String("isp", "", "ISP name filter (ignored when --domain is specified)")
	dnsPullRecordsCmd.Flags().BoolP("yes", "y", false, "Skip confirmation, execute all changes")
	dnsPullRecordsCmd.Flags().Bool("dry-run", false, "Preview changes without executing")
	dnsPullRecordsCmd.Flags().Bool("force", false, "Force overwrite local data even if it already exists")

	dnsPullCmd.AddCommand(dnsPullDomainsCmd)
	dnsPullCmd.AddCommand(dnsPullRecordsCmd)

	return dnsPullCmd
}

type DomainDiff struct {
	Name        string
	ISP         string
	DNSISP      string
	Parent      string
	ChangeType  valueobject.ChangeType
	RecordCount int
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

func runDNSPullDomains(ctx *Context, ispName string, yes, dryRun, force bool) {
	loader := persistence.NewConfigLoader(ctx.ConfigDir)
	cfg, err := loader.Load(nil, ctx.Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to load configuration\n")
		fmt.Fprintf(os.Stderr, "Details: %v\n", err)
		fmt.Fprintf(os.Stderr, "Suggestion: Check that the config directory '%s' and environment '%s' are correct\n", ctx.ConfigDir, ctx.Env)
		os.Exit(1)
	}

	if ispName == "" {
		fmt.Fprintf(os.Stderr, "Error: --isp flag is required\n")
		fmt.Fprintf(os.Stderr, "Details: ISP name is needed to pull domains from a specific provider\n")
		fmt.Fprintf(os.Stderr, "Suggestion: Use --isp <isp_name> to specify ISP (e.g., aliyun, cloudflare)\n")
		os.Exit(1)
	}

	isp := cfg.GetISPMap()[ispName]
	if isp == nil {
		fmt.Fprintf(os.Stderr, "Error: ISP '%s' not found\n", ispName)
		fmt.Fprintf(os.Stderr, "Details: No ISP with name '%s' exists in the configuration\n", ispName)
		fmt.Fprintf(os.Stderr, "Suggestion: Run 'config show isps -e %s' to list available ISPs\n", ctx.Env)
		os.Exit(1)
	}
	if !isp.HasService(entity.ISPServiceDNS) {
		fmt.Fprintf(os.Stderr, "Error: ISP '%s' does not support DNS service\n", ispName)
		fmt.Fprintf(os.Stderr, "Details: ISP '%s' does not have 'dns' in its services list\n", ispName)
		fmt.Fprintf(os.Stderr, "Suggestion: Check ISP configuration in isps.yaml\n")
		os.Exit(1)
	}

	provider, err := createDNSProvider(isp, cfg.GetSecretsMap())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to create DNS provider for ISP '%s'\n", ispName)
		fmt.Fprintf(os.Stderr, "Details: %v\n", err)
		fmt.Fprintf(os.Stderr, "Suggestion: Check ISP credentials and configuration in isps.yaml\n")
		os.Exit(1)
	}

	remoteDomains, err := provider.ListDomains(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to list domains from ISP '%s'\n", ispName)
		fmt.Fprintf(os.Stderr, "Details: %v\n", err)
		fmt.Fprintf(os.Stderr, "Suggestion: Check network connectivity and ISP API credentials\n")
		os.Exit(1)
	}

	remoteRecordCounts := make(map[string]int)
	for _, domainName := range remoteDomains {
		records, err := provider.ListRecords(context.Background(), domainName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to list records for %s: %v\n", domainName, err)
			continue
		}
		remoteRecordCounts[domainName] = len(records)
	}

	localDomainMap := make(map[string]*entity.Domain)
	for i := range cfg.Domains {
		localDomainMap[cfg.Domains[i].Name] = &cfg.Domains[i]
	}

	var diffs []DomainDiff
	for _, domainName := range remoteDomains {
		if local, exists := localDomainMap[domainName]; exists {
			if force {
				diffs = append(diffs, DomainDiff{
					Name:        domainName,
					DNSISP:      ispName,
					ISP:         local.ISP,
					Parent:      local.Parent,
					ChangeType:  valueobject.ChangeTypeUpdate,
					RecordCount: remoteRecordCounts[domainName],
				})
			}
			delete(localDomainMap, domainName)
		} else {
			diffs = append(diffs, DomainDiff{
				Name:        domainName,
				DNSISP:      ispName,
				ChangeType:  valueobject.ChangeTypeCreate,
				RecordCount: remoteRecordCounts[domainName],
			})
		}
	}

	if !force {
		for _, localDomain := range localDomainMap {
			if localDomain.DNSISP == ispName {
				diffs = append(diffs, DomainDiff{
					Name:        localDomain.Name,
					DNSISP:      localDomain.DNSISP,
					ISP:         localDomain.ISP,
					Parent:      localDomain.Parent,
					ChangeType:  valueobject.ChangeTypeDelete,
					RecordCount: len(localDomain.Records),
				})
			}
		}
	}

	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Name < diffs[j].Name
	})

	if len(diffs) == 0 {
		fmt.Println("No domain differences detected.")
		return
	}

	title := buildPlanTitle("dns pull domains", dryRun, force)
	DisplayPlanHeader(PlanHeader{
		Title: title,
		Env:   ctx.Env,
		Extra: []PlanHeaderExtra{{Label: "ISP", Value: ispName}},
	})

	var rows []PlanRow
	for _, diff := range diffs {
		rows = append(rows, PlanRow{
			Action:  formatDomainDiffAction(diff.ChangeType),
			Name:    diff.Name,
			Details: formatDomainDiffDetails(diff),
		})
	}
	DisplayPlanTable3Col("ACTION", "DOMAIN", "DETAILS", rows)
	DisplaySummary(formatSummaryCount("imported", len(diffs)))

	if dryRun {
		DisplayDryRun()
		return
	}

	if yes {
		DisplayExecuting()
		total := len(diffs)
		if err := saveDomainDiffs(ctx, diffs, cfg); err != nil {
			for i, diff := range diffs {
				DisplayExecuteStepWithError(i+1, total, ExecuteItem{
					Action: formatDomainDiffAction(diff.ChangeType),
					Name:   diff.Name,
				}, fmt.Sprintf("Failed to save domains: %v", err), "")
			}
			DisplayResult(0, total)
			return
		}
		for i, diff := range diffs {
			DisplayExecuteStep(i+1, total, ExecuteItem{
				Action:  formatDomainDiffAction(diff.ChangeType),
				Name:    diff.Name,
				Status:  "saved",
				Success: true,
			})
		}
		DisplayResult(total, 0)
		return
	}

	if err := runDomainPullTUI(ctx, diffs, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: TUI interaction failed\n")
		fmt.Fprintf(os.Stderr, "Details: %v\n", err)
		fmt.Fprintf(os.Stderr, "Suggestion: Try using --yes flag to skip TUI and execute directly\n")
		os.Exit(1)
	}
}

func runDNSPullRecords(ctx *Context, domainName string, yes, dryRun, force bool, ispFilter string) {
	loader := persistence.NewConfigLoader(ctx.ConfigDir)
	cfg, err := loader.Load(nil, ctx.Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to load configuration\n")
		fmt.Fprintf(os.Stderr, "Details: %v\n", err)
		fmt.Fprintf(os.Stderr, "Suggestion: Check that the config directory '%s' and environment '%s' are correct\n", ctx.ConfigDir, ctx.Env)
		os.Exit(1)
	}

	var domainsToProcess []entity.Domain
	if domainName != "" {
		domainNames := strings.Split(domainName, ",")
		for _, name := range domainNames {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			domain := cfg.GetDomainMap()[name]
			if domain == nil {
				fmt.Fprintf(os.Stderr, "Error: Domain '%s' not found in local configuration\n", name)
				fmt.Fprintf(os.Stderr, "Details: The specified domain does not exist in the current environment's dns.yaml\n")
				fmt.Fprintf(os.Stderr, "Suggestion: Run 'dns show -e %s' to list available domains\n", ctx.Env)
				os.Exit(1)
			}
			domainsToProcess = append(domainsToProcess, *domain)
		}
		if len(domainsToProcess) == 0 {
			fmt.Fprintf(os.Stderr, "Error: No valid domains specified\n")
			fmt.Fprintf(os.Stderr, "Details: All domain names provided via --domain were empty after trimming\n")
			fmt.Fprintf(os.Stderr, "Suggestion: Provide valid domain names separated by commas (e.g., --domain example.com,api.example.com)\n")
			os.Exit(1)
		}
	} else if ispFilter != "" {
		for _, d := range cfg.Domains {
			if d.DNSISP == ispFilter {
				domainsToProcess = append(domainsToProcess, d)
			}
		}
		if len(domainsToProcess) == 0 {
			fmt.Fprintf(os.Stderr, "Error: No domains found with dns_isp '%s'\n", ispFilter)
			fmt.Fprintf(os.Stderr, "Details: No domain in the current environment is configured to use ISP '%s' as its DNS provider\n", ispFilter)
			fmt.Fprintf(os.Stderr, "Suggestion: Run 'dns show -e %s' to list available domains and their ISPs\n", ctx.Env)
			os.Exit(1)
		}
	} else {
		domainsToProcess = cfg.Domains
	}

	if len(domainsToProcess) == 0 {
		fmt.Println("No domains to pull records from.")
		return
	}

	var allDiffs []RecordDiff
	for _, domain := range domainsToProcess {
		isp := cfg.GetISPMap()[domain.DNSISP]
		if isp == nil {
			fmt.Fprintf(os.Stderr, "Warning: DNS ISP '%s' not found for domain '%s'\n", domain.DNSISP, domain.Name)
			fmt.Fprintf(os.Stderr, "Details: ISP '%s' referenced in dns.yaml does not exist in isps.yaml\n", domain.DNSISP)
			fmt.Fprintf(os.Stderr, "Suggestion: Add ISP '%s' to isps.yaml or update the domain's dns_isp field\n", domain.DNSISP)
			continue
		}

		provider, err := createDNSProvider(isp, cfg.GetSecretsMap())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to create DNS provider for '%s'\n", domain.DNSISP)
			fmt.Fprintf(os.Stderr, "Details: %v\n", err)
			fmt.Fprintf(os.Stderr, "Suggestion: Check ISP credentials for '%s' in isps.yaml\n", domain.DNSISP)
			continue
		}

		remoteRecords, err := provider.ListRecords(context.Background(), domain.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to list records from ISP '%s' for domain '%s'\n", domain.DNSISP, domain.Name)
			fmt.Fprintf(os.Stderr, "Details: %v\n", err)
			fmt.Fprintf(os.Stderr, "Suggestion: Check network connectivity and ISP API credentials for '%s'\n", domain.DNSISP)
			continue
		}

		localRecordMap := make(map[string]*entity.DNSRecord)
		for i := range domain.Records {
			key := fmt.Sprintf("%s:%s:%s", domain.Records[i].Type, domain.Records[i].Name, domain.Records[i].Value)
			localRecordMap[key] = &domain.Records[i]
			localRecordMap[key].Domain = domain.Name
		}

		for _, remote := range remoteRecords {
			recordName := remote.Name
			if recordName == domain.Name || recordName == "" {
				recordName = "@"
			} else if strings.HasSuffix(remote.Name, "."+domain.Name) {
				recordName = strings.TrimSuffix(remote.Name, "."+domain.Name)
			}

			key := fmt.Sprintf("%s:%s:%s", remote.Type, recordName, remote.Value)
			if local, exists := localRecordMap[key]; exists {
				if local.TTL != remote.TTL {
					allDiffs = append(allDiffs, RecordDiff{
						Domain:     domain.Name,
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
				allDiffs = append(allDiffs, RecordDiff{
					Domain:     domain.Name,
					DNSISP:     domain.DNSISP,
					Type:       entity.DNSRecordType(remote.Type),
					Name:       recordName,
					Value:      remote.Value,
					TTL:        remote.TTL,
					ChangeType: valueobject.ChangeTypeCreate,
				})
			}
		}

		if !force {
			for _, local := range localRecordMap {
				allDiffs = append(allDiffs, RecordDiff{
					Domain:     local.Domain,
					DNSISP:     domain.DNSISP,
					Type:       local.Type,
					Name:       local.Name,
					Value:      local.Value,
					TTL:        local.TTL,
					ChangeType: valueobject.ChangeTypeDelete,
				})
			}
		}
	}

	sort.Slice(allDiffs, func(i, j int) bool {
		if allDiffs[i].Domain != allDiffs[j].Domain {
			return allDiffs[i].Domain < allDiffs[j].Domain
		}
		if allDiffs[i].Name != allDiffs[j].Name {
			return allDiffs[i].Name < allDiffs[j].Name
		}
		return allDiffs[i].Type < allDiffs[j].Type
	})

	if len(allDiffs) == 0 {
		fmt.Println("No DNS record differences detected.")
		return
	}

	domainRecordCount := make(map[string]int)
	for _, diff := range allDiffs {
		domainRecordCount[diff.Domain]++
	}

	domainDisplay := domainName
	if domainDisplay == "" {
		domainDisplay = "all"
	}

	title := buildPlanTitle("dns pull records", dryRun, force)
	DisplayPlanHeader(PlanHeader{
		Title: title,
		Env:   ctx.Env,
		Extra: []PlanHeaderExtra{{Label: "DOMAIN", Value: domainDisplay}},
	})

	var rows []PlanRow
	for domain, count := range domainRecordCount {
		rows = append(rows, PlanRow{
			Action:  "import",
			Name:    domain,
			Details: fmt.Sprintf("%d records", count),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	DisplayPlanTable3Col("ACTION", "DOMAIN", "DETAILS", rows)

	DisplaySummary(formatSummaryCount("imported", len(domainRecordCount)))

	if dryRun {
		DisplayDryRun()
		return
	}

	if yes {
		sortedDomains := make([]string, 0, len(domainRecordCount))
		for domain := range domainRecordCount {
			sortedDomains = append(sortedDomains, domain)
		}
		sort.Strings(sortedDomains)

		DisplayExecuting()
		total := len(sortedDomains)
		if err := saveRecordDiffs(ctx, allDiffs, cfg); err != nil {
			for i, domain := range sortedDomains {
				DisplayExecuteStepWithError(i+1, total, ExecuteItem{
					Action: "import",
					Name:   domain,
				}, fmt.Sprintf("Failed to save records: %v", err), "")
			}
			DisplayResult(0, total)
			return
		}
		for i, domain := range sortedDomains {
			DisplayExecuteStep(i+1, total, ExecuteItem{
				Action:  "import",
				Name:    domain,
				Details: fmt.Sprintf("%d records", domainRecordCount[domain]),
				Status:  "saved",
				Success: true,
			})
		}
		DisplayResult(total, 0)
		return
	}

	if err := runRecordPullTUI(ctx, allDiffs, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: TUI interaction failed\n")
		fmt.Fprintf(os.Stderr, "Details: %v\n", err)
		fmt.Fprintf(os.Stderr, "Suggestion: Try using --yes flag to skip TUI and execute directly\n")
		os.Exit(1)
	}
}

func formatDomainDiffAction(changeType valueobject.ChangeType) string {
	switch changeType {
	case valueobject.ChangeTypeCreate:
		return "import"
	case valueobject.ChangeTypeUpdate:
		return "update"
	case valueobject.ChangeTypeDelete:
		return "delete"
	default:
		return "import"
	}
}

func formatDomainDiffDetails(diff DomainDiff) string {
	return fmt.Sprintf("%d records", diff.RecordCount)
}

func formatRecordDiffAction(changeType valueobject.ChangeType) string {
	switch changeType {
	case valueobject.ChangeTypeCreate:
		return "import"
	case valueobject.ChangeTypeUpdate:
		return "update"
	case valueobject.ChangeTypeDelete:
		return "delete"
	default:
		return "import"
	}
}

func createDNSProvider(isp *entity.ISP, secrets map[string]string) (infradns.Provider, error) {
	factory := infradns.NewFactory()
	return factory.Create(isp, secrets)
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
				if diff.Domain == d.Name && string(diff.Type) == string(r.Type) && diff.Name == r.Name &&
					(diff.ChangeType == valueobject.ChangeTypeDelete || diff.ChangeType == valueobject.ChangeTypeUpdate) {
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
	b.WriteString(styles.TitleStyle.Render(title))
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

			prefix, style := styles.FormatChangeType(diff.ChangeType)

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

			prefix, style := styles.FormatChangeType(diff.ChangeType)

			line := fmt.Sprintf("%s [%s] %s %s", cursor, checked, prefix, diff.Name)
			b.WriteString(style.Render(line))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.HelpStyle.Render("↑/k: up  ↓/j: down  space: toggle  a: select all  n: deselect all  enter: confirm  q: quit"))

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
