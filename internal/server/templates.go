package server

import (
	"embed"
	"io/fs"
)

//go:embed templates/apt/*.list
var aptTemplates embed.FS

func GetAPTTemplate(source string) (string, error) {
	data, err := aptTemplates.ReadFile("templates/apt/" + source + ".list")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func ListAPTTemplates() ([]string, error) {
	entries, err := fs.ReadDir(aptTemplates, "templates/apt")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name()[:len(entry.Name())-5])
	}
	return names, nil
}
