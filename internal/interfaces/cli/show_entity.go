package cli

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
)

type FindResult int

const (
	FindResultNotFound FindResult = iota
	FindResultFound
	FindResultUnknownType
)

type EntityFinder func(cfg *entity.Config, name string) (interface{}, FindResult)

type ShowOption func(*showConfig)

type showConfig struct {
	warningMessage string
	typeAliases    map[string]string
	validTypes     []string
}

func WithWarning(msg string) ShowOption {
	return func(c *showConfig) {
		c.warningMessage = msg
	}
}

func WithTypeAliases(aliases map[string]string) ShowOption {
	return func(c *showConfig) {
		c.typeAliases = aliases
	}
}

func WithValidTypes(types []string) ShowOption {
	return func(c *showConfig) {
		c.validTypes = types
	}
}

func showEntity(ctx *Context, entityType, name string, finder EntityFinder, opts ...ShowOption) {
	cfg := showConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	entityType = strings.ToLower(entityType)

	if cfg.typeAliases != nil {
		if aliased, ok := cfg.typeAliases[entityType]; ok {
			entityType = aliased
		}
	}

	wf := NewWorkflow(ctx.Env, ctx.ConfigDir)
	loadedCfg, err := wf.LoadConfig(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	found, result := finder(loadedCfg, name)
	switch result {
	case FindResultUnknownType:
		fmt.Fprintf(os.Stderr, "Unknown entity type: %s\n", entityType)
		if len(cfg.validTypes) > 0 {
			fmt.Fprintf(os.Stderr, "Valid types: %s\n", strings.Join(cfg.validTypes, ", "))
		}
		os.Exit(1)
	case FindResultNotFound:
		fmt.Fprintf(os.Stderr, "%s '%s' not found\n", entityType, name)
		os.Exit(1)
	}

	if cfg.warningMessage != "" {
		fmt.Println(cfg.warningMessage)
		fmt.Println()
	}

	data, err := yaml.Marshal(found)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling entity: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s: %s\n", cases.Title(language.English).String(entityType), name)
	fmt.Println(string(data))
}
