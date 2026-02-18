package cli

type Context struct {
	Env       string
	ConfigDir string
}

func NewContext() *Context {
	return &Context{
		Env:       "dev",
		ConfigDir: ".",
	}
}

type Filters struct {
	Domain  string
	Zone    string
	Server  string
	Service string
}
