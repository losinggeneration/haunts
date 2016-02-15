// +build ignore
package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

var outputTemplate = template.Must(template.New("output").Parse(outputTemplateStr))

const outputTemplateStr = `package main

// This file is auto-generated with go generate

// Version returns the version of Haunts
func Version() string {
	return "{{.}}"
}
`

func read(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func main() {
	head, err := read(filepath.Join(".git", "HEAD"))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	head = strings.TrimSpace(head)

	target := "generate_version.go"
	os.Remove(target) // Don't care about errors on this one
	f, err := os.Create(target)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	outputTemplate.Execute(f, head)
	f.Close()
}
