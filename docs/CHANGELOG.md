# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **`--service` flag**: Service name filter for all `service` subcommands (`show`, `validate`, `deploy`, `stop`, `restart`, `cleanup`). Supports comma-separated multiple values (e.g., `--service my-api,my-worker`)

## [1.0.0] - 2026-06-10

### Added

- **Unified Execution Mode**: All mutation commands now follow `Plan → Confirm → Execute` three-stage workflow
- **`--dry-run` flag**: Preview changes without executing (replaces standalone `plan` command)
- **`--yes` flag**: Skip confirmation and execute all changes (replaces `--auto-approve`)
- **`--force` flag**: Generate deployment plan even when configuration has no changes
- **`--detail` flag**: Show detailed information in `show` commands
- **`--type` flag**: Unified service type filter (`biz` / `infra` / `biz,infra`) for all service subcommands
- **`config validate` command**: Validate ISP/Registry/Secret configuration integrity
- **`server show` command**: List servers with optional `--detail` view
- **`server validate` command**: Validate server configuration
- **`service show` command**: List services with optional `--detail` view
- **`service validate` command**: Validate service configuration with BizService/InfraService uniqueness check
- **`dns show` command**: List domains and DNS records with optional `--detail` view
- **`dns validate` command**: Validate DNS configuration
- **`dns deploy` command**: Deploy DNS changes (replaces `dns apply`)
- **TUI Four-Module Menu**: Service Management / Server Management / DNS Management / Configuration
- **TUI Unified Components**: Plan/Progress/Complete views reused across all mutation operations
- **TUI Four-Zone Layout**: Title bar + Content area + Help bar + Status bar
- **TUI Checkbox Selection**: Plan view with per-item checkbox selection (Space/a/n)
- **TUI Filter Interface**: Type/Zone/Server multi-select filter for stop/restart/cleanup
- **TUI Search Filter**: `/` key to enter search mode in Tree View
- **TUI `--concurrency` parameter**: Control concurrent server operations (default: 5)
- **Multi-select Scope**: All filter dimensions support comma-separated multiple values
- **Exit codes**: 0 (success), 1 (general error), 2 (validation failed), 3 (execution failed)
- **Error format**: `Error:` / `Details:` / `Suggestion:` three-part format

### Changed

- **`-e` flag is now required**: All commands require `-e` parameter (except `--help`)
- **`dns apply` renamed to `dns deploy`**: Consistent naming with service deploy
- **`config show` unified**: `config list` and `config show` merged into `config show` with subcommands (`isps`/`registries`/`secrets`)
- **`--type` semantic change**: Now filters by service category (biz/infra) instead of service name
- **`server setup` unified**: `server check` and `server sync` merged into `server setup` with `--dry-run`/`--yes`
- **Scope refactored**: All filter dimensions now use `[]string` (multi-select), removed `DNSOnly` field
- **DNS-only detection**: Application layer automatically detects DNS-only path based on scope fields
- **TUI menu restructured**: Four modules (Service/Server/DNS/Configuration) instead of two

### Removed

- **`plan` command**: Replaced by `--dry-run` flag on mutation commands
- **`apply` command**: Replaced by `service deploy`
- **`list` command**: Replaced by `show` commands
- **`show` root command**: Replaced by module-specific `show` commands
- **`clean` command**: Replaced by `service cleanup`
- **`env` command**: Replaced by `server setup`
- **`app` command**: Replaced by `service` commands
- **`validate` root command**: Replaced by module-specific `validate` commands
- **`server check` command**: Replaced by `server setup --dry-run`
- **`server sync` command**: Replaced by `server setup --yes`
- **`dns apply` command**: Renamed to `dns deploy`
- **`--auto-approve` flag**: Replaced by `--yes`
- **`--check-only` flag**: Replaced by `--dry-run`
- **`--sync-only` flag**: Replaced by `--yes`
- **`--biz` / `--infra` flags**: Replaced by `--type biz` / `--type infra`
- **`--service <name>` flag**: Now supported as `--service <name>` (comma-separated for multiple services)
- **`DNSOnly` field in Scope**: DNS-only path auto-detected from scope fields
- **TUI fragmented ViewState**: stop/restart/cleanup each had separate Confirm/Progress/Complete views, now unified

### Fixed

- Consistent error handling across all commands
- Unified output format for all `show` commands (table header + data rows + summary)
- Plan output format standardized with ACTION/NAME/SERVER/DETAILS columns
- Execute output format standardized with progress indicators and result summary

### Security

- `config show secrets` only displays key list, never displays values
- `config show isps` never displays API keys or secrets
- All secret references are resolved at runtime, not exposed in output

---

## [0.x.x] - Previous Versions

Previous versions used the old command structure. See [Migration Guide](docs/MIGRATION.md) for upgrade instructions.
