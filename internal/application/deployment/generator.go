package deployment

import (
	"fmt"
	"os"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/generator/compose"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/generator/gate"
)

type Generator struct {
	composeGen *compose.Generator
	gateGen    *gate.Generator
	outputDir  string
	env        string
}

func NewGenerator(env, outputDir string) *Generator {
	return &Generator{
		composeGen: compose.NewGenerator(),
		gateGen:    gate.NewGenerator(),
		outputDir:  outputDir,
		env:        env,
	}
}

func (g *Generator) SetOutputDir(dir string) {
	g.outputDir = dir
}

func (g *Generator) Generate(config *entity.Config) error {
	if _, err := os.Stat(g.outputDir); err == nil {
		if err := os.RemoveAll(g.outputDir); err != nil {
			return fmt.Errorf("failed to clean output directory: %w", err)
		}
	}

	if err := os.MkdirAll(g.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := g.generateServiceComposes(config); err != nil {
		return err
	}

	if err := g.generateGatewayConfigs(config); err != nil {
		return err
	}

	if err := g.generateInfraServiceComposes(config); err != nil {
		return err
	}

	return nil
}
