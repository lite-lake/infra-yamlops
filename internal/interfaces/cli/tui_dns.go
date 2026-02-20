package cli

import (
	"fmt"
	"strings"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

func (m Model) renderDNSManagement() string {
	var sb strings.Builder

	title := TitleStyle.Render("  DNS Management")
	sb.WriteString(title + "\n\n")

	items := []string{
		"Pull Domains        Pull domain list from ISP",
		"Pull Records        Pull DNS records from domain",
		"Plan & Apply        DNS change plan/apply",
		"Back to Menu        Return to main menu",
	}

	for i, item := range items {
		if i == m.DNS.DNSMenuIndex {
			sb.WriteString(MenuSelectedStyle.Render("> "+item) + "\n")
		} else {
			sb.WriteString(MenuItemStyle.Render("  "+item) + "\n")
		}
	}

	sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ navigate  Enter select  Esc back  q quit"))

	return BaseStyle.Render(sb.String())
}

func (m Model) renderDNSPullDomains() string {
	isps := m.getDNSISPs()

	availableHeight := m.UI.Height - 8
	if availableHeight < 5 {
		availableHeight = 5
	}

	viewport := NewViewport(0, len(isps), availableHeight)
	viewport.CursorIndex = m.DNS.DNSISPIndex
	viewport.EnsureCursorVisible()

	var sb strings.Builder
	title := TitleStyle.Render("  Pull Domains - Select ISP")
	sb.WriteString(title + "\n\n")

	if len(isps) == 0 {
		sb.WriteString("No ISPs with DNS service configured.\n")
	} else {
		for i := viewport.VisibleStart(); i < viewport.VisibleEnd() && i < len(isps); i++ {
			if i == m.DNS.DNSISPIndex {
				sb.WriteString(MenuSelectedStyle.Render("> "+isps[i]) + "\n")
			} else {
				sb.WriteString(MenuItemStyle.Render("  "+isps[i]) + "\n")
			}
		}
	}

	sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ navigate  Enter select  Esc back  q quit"))

	if viewport.TotalRows > viewport.VisibleRows {
		sb.WriteString("\n" + viewport.RenderSimpleScrollIndicator())
	}

	return BaseStyle.Render(sb.String())
}

func (m Model) getDNSISPs() []string {
	var isps []string
	for _, isp := range m.Config.ISPs {
		if isp.HasService(entity.ISPServiceDNS) {
			isps = append(isps, isp.Name)
		}
	}
	return isps
}

func (m Model) getDNSDomains() []string {
	var domains []string
	if m.Config == nil || m.Config.Domains == nil {
		return domains
	}
	for _, d := range m.Config.Domains {
		domains = append(domains, d.Name)
	}
	return domains
}

func (m Model) renderDNSPullRecords() string {
	domains := m.getDNSDomains()

	availableHeight := m.UI.Height - 8
	if availableHeight < 5 {
		availableHeight = 5
	}

	viewport := NewViewport(0, len(domains), availableHeight)
	viewport.CursorIndex = m.DNS.DNSDomainIndex
	viewport.EnsureCursorVisible()

	var sb strings.Builder
	title := TitleStyle.Render("  Pull Records - Select Domain")
	sb.WriteString(title + "\n\n")

	if len(domains) == 0 {
		sb.WriteString("No domains configured.\n")
	} else {
		for i := viewport.VisibleStart(); i < viewport.VisibleEnd() && i < len(domains); i++ {
			if i == m.DNS.DNSDomainIndex {
				sb.WriteString(MenuSelectedStyle.Render("> "+domains[i]) + "\n")
			} else {
				sb.WriteString(MenuItemStyle.Render("  "+domains[i]) + "\n")
			}
		}
	}

	sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ navigate  Enter select  Esc back  q quit"))

	if viewport.TotalRows > viewport.VisibleRows {
		sb.WriteString("\n" + viewport.RenderSimpleScrollIndicator())
	}

	return BaseStyle.Render(sb.String())
}

func (m Model) renderDNSPullDiff() string {
	availableHeight := m.UI.Height - 8
	if availableHeight < 5 {
		availableHeight = 5
	}

	if m.UI.ErrorMessage != "" {
		var sb strings.Builder
		sb.WriteString(TitleStyle.Render("  Error") + "\n\n")
		sb.WriteString(ChangeDeleteStyle.Render("  "+m.UI.ErrorMessage) + "\n")
		sb.WriteString("\n" + HelpStyle.Render("  Esc back  q quit"))
		return BaseStyle.Render(sb.String())
	}

	if len(m.DNS.DNSPullDiffs) > 0 {
		viewport := NewViewport(0, len(m.DNS.DNSPullDiffs), availableHeight)
		viewport.CursorIndex = m.DNS.DNSPullCursor
		viewport.EnsureCursorVisible()

		var sb strings.Builder
		title := TitleStyle.Render("  Select Domains to Sync")
		sb.WriteString(title + "\n\n")

		for i := viewport.VisibleStart(); i < viewport.VisibleEnd() && i < len(m.DNS.DNSPullDiffs); i++ {
			diff := m.DNS.DNSPullDiffs[i]
			cursor := " "
			if m.DNS.DNSPullCursor == i {
				cursor = ">"
			}
			checked := " "
			if m.DNS.DNSPullSelected[i] {
				checked = "x"
			}

			var prefix string
			var style = ChangeNoopStyle
			switch diff.ChangeType {
			case valueobject.ChangeTypeCreate:
				prefix = "+"
				style = ChangeCreateStyle
			case valueobject.ChangeTypeDelete:
				prefix = "-"
				style = ChangeDeleteStyle
			}

			line := fmt.Sprintf("%s [%s] %s %s", cursor, checked, prefix, diff.Name)
			sb.WriteString(style.Render(line) + "\n")
		}

		sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ move  Space toggle  a all  n none  Enter confirm  Esc cancel  q quit"))

		if viewport.TotalRows > viewport.VisibleRows {
			sb.WriteString("\n" + viewport.RenderScrollIndicator())
		}

		return BaseStyle.Render(sb.String())
	} else if len(m.DNS.DNSRecordDiffs) > 0 {
		viewport := NewViewport(0, len(m.DNS.DNSRecordDiffs), availableHeight)
		viewport.CursorIndex = m.DNS.DNSPullCursor
		viewport.EnsureCursorVisible()

		var sb strings.Builder
		title := TitleStyle.Render("  Select DNS Records to Sync")
		sb.WriteString(title + "\n\n")

		for i := viewport.VisibleStart(); i < viewport.VisibleEnd() && i < len(m.DNS.DNSRecordDiffs); i++ {
			diff := m.DNS.DNSRecordDiffs[i]
			cursor := " "
			if m.DNS.DNSPullCursor == i {
				cursor = ">"
			}
			checked := " "
			if m.DNS.DNSPullSelected[i] {
				checked = "x"
			}

			var prefix string
			var style = ChangeNoopStyle
			switch diff.ChangeType {
			case valueobject.ChangeTypeCreate:
				prefix = "+"
				style = ChangeCreateStyle
			case valueobject.ChangeTypeUpdate:
				prefix = "~"
				style = ChangeUpdateStyle
			case valueobject.ChangeTypeDelete:
				prefix = "-"
				style = ChangeDeleteStyle
			}

			line := fmt.Sprintf("%s [%s] %s %-6s %-20s -> %-30s",
				cursor, checked, prefix, diff.Type, diff.Name, diff.Value)
			sb.WriteString(style.Render(line) + "\n")
		}

		sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ move  Space toggle  a all  n none  Enter confirm  Esc cancel  q quit"))

		if viewport.TotalRows > viewport.VisibleRows {
			sb.WriteString("\n" + viewport.RenderScrollIndicator())
		}

		return BaseStyle.Render(sb.String())
	} else {
		var sb strings.Builder
		sb.WriteString(TitleStyle.Render("  No Differences") + "\n\n")
		sb.WriteString("All items are in sync.\n")
		sb.WriteString("\n" + HelpStyle.Render("  Esc back  q quit"))
		return BaseStyle.Render(sb.String())
	}
}
