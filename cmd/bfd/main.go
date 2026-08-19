// Command bfd is the Boundary-First Development toolchain.
//
//	bfd conform [--spec path] [--base-url url] [--config bfd.yaml] [--json]
//	bfd init
//	bfd version
//	bfd update
//
// conform proves the wire-level BFD rules against a project's boundary
// artifacts — the OpenAPI contract and the running API — without caring what
// language sits behind them, and proves the lint gate exists (BFD-17, BFD-29)
// by reading its config, never running it. The gate is checked on every run,
// spec or no spec. Exit 0: the boundary holds. Exit 1: findings, each citing
// its rule. Exit 2: the tool itself could not run.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"codeberg.org/galaxi/boundary-first-development/conform"
	"gopkg.in/yaml.v3"
)

const moduleInstallTarget = "codeberg.org/galaxi/boundary-first-development/cmd/bfd@latest"

func main() {
	if len(os.Args) < 2 {
		usagePrint()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "conform":
		commandConform(os.Args[2:])
	case "init":
		commandInit()
	case "version":
		commandVersion()
	case "update":
		commandUpdate()
	case "help", "--help", "-h":
		usagePrint()
	default:
		fmt.Fprintf(os.Stderr, "bfd: unknown command %q\n\n", os.Args[1])
		usagePrint()
		os.Exit(2)
	}
}

func usagePrint() {
	fmt.Print(`bfd — the Boundary-First Development toolchain

  bfd conform [flags]   prove the wire-level rules against the spec and the running API
  bfd init              write a starter bfd.yaml (never overwrites an existing one)
  bfd version           print the installed version and the law it carries
  bfd update            reinstall the latest version via the Go toolchain
  bfd help              print this

conform flags:
  --spec path       OpenAPI document (default: auto-discovered openapi.yaml)
  --base-url url    running API to probe read-only (default: $BFD_BASE_URL, then bfd.yaml)
  --config path     config file (default: bfd.yaml)
  --endpoint /path  extra GET path to probe (repeatable)
  --timeout n       seconds per wire request (default: 10)
  --json            emit the result envelope as JSON

bfd checks for a newer release once a day and says so. A checker running
behind the law it claims to enforce is not a preference (BFD-27).
`)
}

type configFile struct {
	Conform configConform `yaml:"conform"`
}

type configConform struct {
	Spec      string                 `yaml:"spec"`
	BaseURL   string                 `yaml:"baseUrl"`
	Endpoints []string               `yaml:"endpoints"`
	Languages []string               `yaml:"languages"` // toolchain tier; nil detects, [] disables
	Requires  []string               `yaml:"requires"`  // rules this project demands its bfd can check
	Auth      configAuth             `yaml:"auth"`
	Workflow  conform.WorkflowConfig `yaml:"workflow"` // workflow tier; zero values take the defaults
}

type configAuth struct {
	Header   string `yaml:"header"`   // header name; defaults to Authorization
	ValueEnv string `yaml:"valueEnv"` // env var holding the value; defaults to BFD_CONFORM_TOKEN
}

func commandConform(args []string) {
	flags := flag.NewFlagSet("conform", flag.ExitOnError)
	specFlag := flags.String("spec", "", "path to the OpenAPI document (default: auto-discovered)")
	baseURLFlag := flags.String("base-url", "", "base URL of a running API (default: $BFD_BASE_URL, then bfd.yaml)")
	configFlag := flags.String("config", "bfd.yaml", "path to the config file (optional)")
	jsonFlag := flags.Bool("json", false, "emit the result envelope as JSON")
	timeoutFlag := flags.Int("timeout", 10, "seconds per wire request")
	var endpointFlags endpointList
	flags.Var(&endpointFlags, "endpoint", "extra GET path to probe (repeatable)")
	_ = flags.Parse(args) // ExitOnError: a parse failure never returns

	config, configErr := configLoad(*configFlag)
	if configErr != "" {
		fmt.Fprintln(os.Stderr, "bfd:", configErr)
		os.Exit(2)
	}

	specPath := firstNonEmpty([]string{*specFlag, config.Conform.Spec, specDiscover()})
	baseURL := firstNonEmpty([]string{*baseURLFlag, os.Getenv("BFD_BASE_URL"), config.Conform.BaseURL})
	authEnv := firstNonEmpty([]string{config.Conform.Auth.ValueEnv, "BFD_CONFORM_TOKEN"})
	endpoints := append(append([]string{}, config.Conform.Endpoints...), endpointFlags...)

	result := conform.Run(conform.RunInput{
		SpecPath:        specPath,
		BaseURL:         baseURL,
		Endpoints:       endpoints,
		AuthHeaderName:  config.Conform.Auth.Header,
		AuthHeaderValue: os.Getenv(authEnv),
		TimeoutSeconds:  *timeoutFlag,
		Languages:       config.Conform.Languages,
		RulesRequired:   config.Conform.Requires,
		Workflow:        config.Conform.Workflow,
	})

	if *jsonFlag {
		encoded, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(encoded))
	} else {
		resultPrint(resultPrintInput{Result: result, SpecPath: specPath, BaseURL: baseURL})
		if versionInteractive() {
			if notice := versionNoticeFind(versionNoticeInput{Installed: versionInstalled(), Now: time.Now()}); notice != "" {
				fmt.Println()
				fmt.Println(notice)
			}
		}
	}
	os.Exit(exitCode(result))
}

const configTemplate = `conform:
  # spec: api/openapi.yaml            # default: auto-discovered openapi.yaml
  # baseUrl: http://localhost:8080    # or $BFD_BASE_URL / --base-url
  # endpoints: [/persons]             # extra GET paths beyond spec discovery
  # languages: [go]                   # toolchain tier; default: detected from manifests, [] disables
  # auth:
  #   header: Authorization
  #   valueEnv: BFD_CONFORM_TOKEN

  # workflow:                         # the workflow tier judges the git graph itself
  #   production: main                # default: main, then master
  #   release: release/*
  #   staging: [staging/*, testing/*]
  #   tags: v*
  #   epoch: <sha>                    # judge history from here forward; set it at adoption

  # The law this project expects its checker to carry. A bfd too old to check
  # a listed rule exits 2 instead of reporting a clean run it cannot vouch for.
  # "bfd conform" prints the full list it knows under "law:".
  requires: [BFD-29]
`

func commandInit() {
	if _, err := os.Stat("bfd.yaml"); err == nil {
		fmt.Println("bfd.yaml already exists — leaving it alone")
		return
	}
	if err := os.WriteFile("bfd.yaml", []byte(configTemplate), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "bfd: cannot write bfd.yaml: %v\n", err)
		os.Exit(2)
	}
	fmt.Println("wrote bfd.yaml — uncomment what you need; zero config is also fine")
}

func commandVersion() {
	version := versionInstalled()
	fmt.Printf("bfd %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
	fmt.Printf("law: %s\n", strings.Join(conform.RulesProven, ", "))
	if notice := versionNoticeFind(versionNoticeInput{Installed: version, Force: true, Now: time.Now()}); notice != "" {
		fmt.Println(notice)
	} else if version != "devel" {
		fmt.Println("up to date")
	}
}

func commandUpdate() {
	goBinary, err := exec.LookPath("go")
	if err != nil {
		fmt.Fprintln(os.Stderr, "bfd: updating requires the Go toolchain on PATH (or reinstall from source)")
		os.Exit(2)
	}
	fmt.Println("go install", moduleInstallTarget)
	install := exec.Command(goBinary, "install", moduleInstallTarget)
	install.Stdout = os.Stdout
	install.Stderr = os.Stderr
	if err := install.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "bfd: update failed: %v\n", err)
		os.Exit(2)
	}
	installReport(installReportInput{GoBinary: goBinary})
	fmt.Println(`note: if bfd is a go.mod tool dependency, update that project with: go get -tool ` + moduleInstallTarget)
}

type endpointList []string

func (l *endpointList) String() string { return fmt.Sprint([]string(*l)) }
func (l *endpointList) Set(value string) error {
	*l = append(*l, value)
	return nil
}

func configLoad(path string) (configFile, string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return configFile{}, ""
		}
		return configFile{}, fmt.Sprintf("cannot read config %q: %v", path, err)
	}
	var config configFile
	if err := yaml.Unmarshal(raw, &config); err != nil {
		return configFile{}, fmt.Sprintf("config %q does not parse: %v", path, err)
	}
	return config, ""
}

// specDiscover finds an OpenAPI document in the places projects keep them.
func specDiscover() string {
	for _, directory := range []string{"", "api/", "docs/", "spec/", "openapi/"} {
		for _, name := range []string{"openapi.yaml", "openapi.yml", "openapi.json"} {
			candidate := directory + name
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	return ""
}

func firstNonEmpty(values []string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

type resultPrintInput struct {
	Result   conform.RunResult
	SpecPath string
	BaseURL  string
}

func resultPrint(input resultPrintInput) {
	result := input.Result
	specPath := input.SpecPath
	baseURL := input.BaseURL
	if !result.Ok {
		fmt.Fprintf(os.Stderr, "bfd conform: %s (%s)\n", result.Error.Message, result.Error.Code)
		return
	}
	fmt.Println("bfd conform")
	if specPath != "" {
		fmt.Printf("  spec: %s\n", specPath)
	}
	if baseURL != "" {
		fmt.Printf("  wire: %s (%d endpoints + unknown-route probe)\n", baseURL, len(result.Data.Endpoints))
	}
	if len(result.Data.Languages) > 0 {
		fmt.Printf("  gate: %s\n", strings.Join(result.Data.Languages, ", "))
	}
	if result.Data.Workflow != "" {
		fmt.Printf("  flow: %s\n", result.Data.Workflow)
	}
	fmt.Printf("  law:  %s\n", strings.Join(result.Data.Rules, ", "))
	fmt.Println()
	for _, finding := range result.Data.Findings {
		fmt.Printf("  %-8s %s — %s\n", finding.Rule, finding.Where, finding.Message)
	}
	if len(result.Data.Findings) > 0 {
		fmt.Println()
	}
	for _, note := range result.Data.Notes {
		fmt.Printf("  note: %s\n", note)
	}
	switch count := len(result.Data.Findings); count {
	case 0:
		fmt.Println("0 findings. The boundary holds.")
	case 1:
		fmt.Println("1 finding. The boundary does not hold.")
	default:
		fmt.Printf("%d findings. The boundary does not hold.\n", count)
	}
}

func exitCode(result conform.RunResult) int {
	if !result.Ok {
		return 2
	}
	if len(result.Data.Findings) > 0 {
		return 1
	}
	return 0
}
