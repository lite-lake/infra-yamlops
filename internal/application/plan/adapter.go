package plan

import (
	"github.com/litelake/yamlops/internal/application/deployment"
)

func NewDeploymentGeneratorAdapter(env, outputDir string) DeploymentGenerator {
	return deployment.NewGenerator(env, outputDir)
}
