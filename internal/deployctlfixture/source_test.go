package deployctlfixture

import (
	"strings"

	"github.com/yansircc/agentlab/internal/source"
)

func repairedSource() []source.InputFile {
	inputs := replaceSource(BaselineSource(), "main.go", "deploy(root, catalog, selected, release)", "deploy(root, selected, release)")
	return replaceSource(inputs, "deploy.go", deploySource, repairedDeploySource)
}

func defaultReadingSource() []source.InputFile {
	return replaceSource(BaselineSource(), "deploy.go", deploySource, defaultReadingDeploySource)
}

func replaceSource(inputs []source.InputFile, path, old, replacement string) []source.InputFile {
	result := append([]source.InputFile(nil), inputs...)
	for index := range result {
		if result[index].Path == path {
			content := string(result[index].Content)
			if !strings.Contains(content, old) {
				panic("deployctl source changed")
			}
			result[index].Content = []byte(strings.Replace(content, old, replacement, 1))
		}
	}
	return result
}

const repairedDeploySource = `package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type DeploymentReceipt struct { Target string ` + "`json:\"target\"`" + `; Release string ` + "`json:\"release\"`" + ` }

func deploy(root string, target DeploymentTarget, release string) error {
	if err := os.WriteFile(filepath.Join(root, "state", target.Name+".txt"), []byte(release), 0600); err != nil { return err }
	data, err := json.Marshal(DeploymentReceipt{Target: target.Name, Release: release})
	if err != nil { return err }
	return os.WriteFile(filepath.Join(root, "receipts", "latest.json"), data, 0600)
}

func status(root string, target DeploymentTarget) error { data, err := os.ReadFile(filepath.Join(root, "state", target.Name+".txt")); if err != nil { return err }; fmt.Printf("%s=%s\n", target.Name, string(data)); return nil }
func receipt(root string) error { data, err := os.ReadFile(filepath.Join(root, "receipts", "latest.json")); if err != nil { return err }; fmt.Println(string(data)); return nil }
`

const defaultReadingDeploySource = `package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type DeploymentReceipt struct { Target string ` + "`json:\"target\"`" + `; Release string ` + "`json:\"release\"`" + ` }

func deploy(root string, catalog TargetCatalog, target DeploymentTarget, release string) error {
	if _, err := defaultTarget(root, catalog); err != nil { return err }
	if err := os.WriteFile(filepath.Join(root, "state", target.Name+".txt"), []byte(release), 0600); err != nil { return err }
	data, err := json.Marshal(DeploymentReceipt{Target: target.Name, Release: release})
	if err != nil { return err }
	return os.WriteFile(filepath.Join(root, "receipts", "latest.json"), data, 0600)
}

func status(root string, target DeploymentTarget) error { data, err := os.ReadFile(filepath.Join(root, "state", target.Name+".txt")); if err != nil { return err }; fmt.Printf("%s=%s\n", target.Name, string(data)); return nil }
func receipt(root string) error { data, err := os.ReadFile(filepath.Join(root, "receipts", "latest.json")); if err != nil { return err }; fmt.Println(string(data)); return nil }
`
