package generator

type Generator interface {
	Generate(config interface{}, opts ...interface{}) (string, error)
}
