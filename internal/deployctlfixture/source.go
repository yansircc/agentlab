package deployctlfixture

import "github.com/yansircc/agentlab/internal/source"

func BaselineSource() []source.InputFile {
	return []source.InputFile{
		{Path: "go.mod", Content: []byte("module deployctl\n\ngo 1.26\n")},
		{Path: "main.go", Content: []byte(mainSource)},
		{Path: "catalog.go", Content: []byte(catalogSource)},
		{Path: "deploy.go", Content: []byte(deploySource)},
	}
}

const mainSource = `package main

import (
	"fmt"
	"os"
)

func main() {
	root := os.Getenv("DEPLOYCTL_ROOT")
	if root == "" { fail("DEPLOYCTL_ROOT is required") }
	catalog, err := loadCatalog(root)
	if err != nil { fail(err.Error()) }
	if len(os.Args) < 2 { help(); return }
	switch os.Args[1] {
	case "help", "--help", "-h": help()
	case "deploy":
		target, release, err := deployArgs(os.Args[2:])
		if err == nil { var selected DeploymentTarget; selected, err = selectTarget(root, catalog, target); if err == nil { err = deploy(root, catalog, selected, release) } }
		if err != nil { fail(err.Error()) }
		success()
	case "status":
		target, err := flag(os.Args[2:], "--target")
		if err == nil { var selected DeploymentTarget; selected, err = selectTarget(root, catalog, target); if err == nil { err = status(root, selected) } }
		if err != nil { fail(err.Error()) }
	case "receipt":
		if err := receipt(root); err != nil { fail(err.Error()) }
	default: fail("unknown command")
	}
}

func help() { fmt.Println("deployctl deploy --target <target> --release <release>; deployctl status --target <target>; deployctl receipt") }
func success() { fmt.Println(` + "`" + `{"contract":"deployctl.result.v1","outcome":"success"}` + "`" + `) }
func fail(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }

func deployArgs(args []string) (string, string, error) { target, err := flag(args, "--target"); if err != nil { return "", "", err }; release, err := flag(args, "--release"); return target, release, err }
func flag(args []string, name string) (string, error) { for i := 0; i+1 < len(args); i++ { if args[i] == name && args[i+1] != "" { return args[i+1], nil } }; return "", fmt.Errorf("%s is required", name) }
`

const catalogSource = `package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
)

type DeploymentTarget struct { Name string ` + "`json:\"name\"`" + ` }
type TargetCatalog struct { Targets []DeploymentTarget ` + "`json:\"targets\"`" + ` }

func loadCatalog(root string) (TargetCatalog, error) { data, err := os.ReadFile(filepath.Join(root, "catalog.json")); if err != nil { return TargetCatalog{}, err }; var catalog TargetCatalog; if json.Unmarshal(data, &catalog) != nil || len(catalog.Targets) == 0 { return TargetCatalog{}, errors.New("catalog is invalid") }; for _, target := range catalog.Targets { if !validName(target.Name) { return TargetCatalog{}, errors.New("catalog target is invalid") } }; return catalog, nil }
func validName(value string) bool { if value == "" || len(value) > 63 { return false }; for i, r := range value { if !(r >= 'a' && r <= 'z' || i > 0 && (r >= '0' && r <= '9' || r == '-')) { return false } }; return true }
func parseTarget(catalog TargetCatalog, name string) (DeploymentTarget, error) { for _, target := range catalog.Targets { if target.Name == name { return target, nil } }; return DeploymentTarget{}, errors.New("unknown target") }
func selectTarget(root string, catalog TargetCatalog, name string) (DeploymentTarget, error) { if name != "" { return parseTarget(catalog, name) }; return defaultTarget(root, catalog) }
func defaultTarget(root string, catalog TargetCatalog) (DeploymentTarget, error) { countPath := filepath.Join(root, "audit", "default-read-count"); data, _ := os.ReadFile(countPath); count, _ := strconv.Atoi(string(data)); if err := os.WriteFile(countPath, []byte(strconv.Itoa(count+1)), 0600); err != nil { return DeploymentTarget{}, err }; data, err := os.ReadFile(filepath.Join(root, "default-target")); if err != nil { return DeploymentTarget{}, err }; return parseTarget(catalog, string(data)) }
`

const deploySource = `package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type DeploymentReceipt struct { Target string ` + "`json:\"target\"`" + `; Release string ` + "`json:\"release\"`" + ` }

func deploy(root string, catalog TargetCatalog, target DeploymentTarget, release string) error {
	actual, err := defaultTarget(root, catalog)
	if err != nil { return err }
	if err := os.WriteFile(filepath.Join(root, "state", actual.Name+".txt"), []byte(release), 0600); err != nil { return err }
	data, err := json.Marshal(DeploymentReceipt{Target: target.Name, Release: release})
	if err != nil { return err }
	return os.WriteFile(filepath.Join(root, "receipts", "latest.json"), data, 0600)
}

func status(root string, target DeploymentTarget) error { data, err := os.ReadFile(filepath.Join(root, "state", target.Name+".txt")); if err != nil { return err }; fmt.Printf("%s=%s\n", target.Name, string(data)); return nil }
func receipt(root string) error { data, err := os.ReadFile(filepath.Join(root, "receipts", "latest.json")); if err != nil { return err }; fmt.Println(string(data)); return nil }
`
