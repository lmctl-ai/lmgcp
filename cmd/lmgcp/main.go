// SPDX-License-Identifier: MIT

package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/lmctl-ai/lmcode-go"
	"github.com/lmctl-ai/lmgcp/internal/host"
	"github.com/lmctl-ai/lmgcp/internal/invoke"
	"github.com/lmctl-ai/lmgcp/internal/membudget"
	"github.com/lmctl-ai/lmgcp/internal/safety"
	"github.com/lmctl-ai/lmgcp/schema"
)

//go:embed VERSION
var rawVersion string

//go:embed THIRD_PARTY_NOTICES
var embeddedNotices string

const usageText = `usage: lmgcp --version | --help | --notices | --lmcode-schema [--json] | --lmcode-signatures [--without-std] | [--lmcode|--first|--last] FILE [FILE ...] | --sweep [--older-than-hours N] [--delete] [--run-id ID] [--ledger PATH]

Google Cloud Go SDK host for lmscript (Apache-2.0 SDK on lmcode-go).

Options:
  --version                  Print version (lmgcp X.Y.Z)
  --help, -h                 Print this usage and safety overview
  --notices                  Print embedded third-party license notices
  --lmcode-schema [--json]      Print human-readable schema summary or raw JSON schema
  --lmcode-signatures           Print sorted canonical DSL signatures
  --lmcode|--first|--last FILE...  Load, verify, and execute compiled .dslb files
  --sweep                    List (dry-run default) or delete (--delete) run-owned leftovers
                             --run-id ID / --ledger PATH reload a durable recovery journal

Dry mode comes from the lmcode invocation manifest; there is no --dry-run flag.

Environment & Resource Limits:
  LMGCP_MAX_MEMORY_MB        Total memory budget in whole MiB (default: 4096; invalid = exit 2)
  LMGCP_LIVE                 Set to 1 to use Google Application Default Credentials (default: 0, in-process fake GCP)
  LMGCP_ENDPOINT            Loopback HTTP emulator origin (mutually exclusive with live mode)
  LMGCP_ACCESS_TOKEN        Optional dummy bearer token for the emulator
  LMGCP_PROJECT              Target GCP project ID (masked in all outputs)
  LMGCP_REGION               Default GCP region (default: us-central1)
  LMGCP_ZONE                 Default GCP zone (default: us-central1-a)
  LMGCP_STATE_DIR            Durable recovery journal directory (default: user state dir)
  LMGCP_LEDGER               Explicit recovery journal path (overrides LMGCP_STATE_DIR)

Linux Runtime Note (Rule 14):
  Built with CGO_ENABLED=0 (or dynamic glibc >= 2.35 on Linux); zero GPL/LGPL code linked.
`

func printSchemaSummary(reg *lmcode.Registry) {
	current := ""
	var order []string
	members := make(map[string][]string)
	for _, sig := range reg.Signatures() {
		if strings.HasPrefix(sig, "context ") || strings.HasPrefix(sig, "fn ") {
			fmt.Println(sig)
		} else if strings.HasPrefix(sig, "method ") {
			body := sig[7:]
			idx := strings.Index(body, ".")
			if idx > 0 {
				typeName := body[:idx]
				if typeName != current {
					current = typeName
					order = append(order, typeName)
				}
				members[typeName] = append(members[typeName], body)
			}
		}
	}
	for _, t := range order {
		fmt.Printf("%s:\n", t)
		for _, m := range members[t] {
			fmt.Printf("  %s\n", m)
		}
	}
}

func main() {
	os.Exit(realMain(os.Args))
}

func realMain(args []string) int {
	if len(args) >= 2 && args[1] == "cloud" {
		return cloudRealMain(args[2:])
	}
	if _, err := membudget.InitFromEnv(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return invoke.Failure("cli", lmcode.NewError(lmcode.CLI_USAGE, err.Error()))
	}

	if len(args) == 2 && (args[1] == "--help" || args[1] == "-h") {
		fmt.Print(usageText)
		return 0
	}

	if len(args) == 2 && args[1] == "--version" {
		v := strings.TrimSpace(rawVersion)
		if v == "" {
			v = "0.1.0"
		}
		fmt.Printf("lmgcp %s\n", v)
		return 0
	}

	if len(args) == 2 && args[1] == "--notices" {
		fmt.Print(embeddedNotices)
		return 0
	}

	// Reject unrecognized operations before starting the fake or live transport.
	invocation := len(args) >= 2 && (args[1] == "--lmcode" || args[1] == "--first" || args[1] == "--last")
	if len(args) < 2 || (args[1] != "--lmcode-schema" && args[1] != "--lmcode-signatures" && !invocation && args[1] != "--sweep") {
		fmt.Fprint(os.Stderr, usageText)
		return invoke.Failure("cli", lmcode.NewError(lmcode.CLI_USAGE, usageText))
	}

	var results *lmcode.InvocationResults
	var order []int
	builderInvocation := false
	if invocation {
		type part struct {
			file     string
			priority int
			index    int
		}
		parts := make([]part, 0, len(args)/2)
		files := make([]string, 0, len(args)/2)
		add := func(file string, priority int) bool {
			if file == "" || strings.HasPrefix(file, "--") {
				return false
			}
			parts = append(parts, part{file: file, priority: priority, index: len(parts)})
			files = append(files, file)
			return true
		}
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--lmcode", "--first", "--last":
				priority := 1
				if args[i] == "--first" {
					priority = 0
				} else if args[i] == "--last" {
					priority = 2
				}
				if i+1 >= len(args) || !add(args[i+1], priority) {
					return invoke.Failure("cli", lmcode.NewError(lmcode.CLI_USAGE, usageText))
				}
				i++
			default:
				if !add(args[i], 1) {
					return invoke.Failure("cli", lmcode.NewError(lmcode.CLI_USAGE, usageText))
				}
			}
		}
		if len(parts) == 0 {
			return invoke.Failure("cli", lmcode.NewError(lmcode.CLI_USAGE, usageText))
		}
		collected, err := lmcode.InvocationResultsFromCommand(files)
		if err != nil {
			return invoke.Failure("cli", err)
		}
		results = collected
		order = make([]int, len(parts))
		builderInvocation = len(parts) > 1
		if manifest := os.Getenv("LMCODE_INVOCATION_FILE"); manifest != "" {
			var meta struct {
				Mode string `json:"mode"`
			}
			if raw, err := os.ReadFile(manifest); err == nil && json.Unmarshal(raw, &meta) == nil {
				builderInvocation = meta.Mode == "builder"
			}
		}
		for i := range parts {
			order[i] = i
		}
		sort.SliceStable(order, func(i, j int) bool {
			a, b := parts[order[i]], parts[order[j]]
			if a.priority != b.priority {
				return a.priority < b.priority
			}
			return a.index < b.index
		})
	}
	dryInvocation := results != nil && results.Dry()

	var h *host.Host
	var err error
	if args[1] == "--sweep" {
		h, err = host.NewHostForSweep()
	} else {
		h, err = host.NewHost(dryInvocation)
	}
	if err != nil {
		return invoke.Failure("app", lmcode.NewError(lmcode.APP_FAIL, "failed to initialize GCP host: "+safety.RedactSecrets(err.Error())))
	}
	defer h.Close()

	reg := lmcode.NewRegistry()
	if err := host.Register(reg, h); err != nil {
		return invoke.Failure("app", lmcode.NewError(lmcode.APP_FAIL, "failed to register host: "+err.Error()))
	}

	if args[1] == "--lmcode-schema" {
		if len(args) == 2 {
			printSchemaSummary(reg)
			return 0
		}
		if len(args) == 3 && args[2] == "--json" {
			if _, err := os.Stdout.Write(schema.EmbeddedSchema); err != nil {
				return 4
			}
			return 0
		}
		fmt.Fprint(os.Stderr, usageText)
		return invoke.Failure("cli", lmcode.NewError(lmcode.CLI_USAGE, usageText))
	}

	if len(args) == 3 && args[1] == "--lmcode-signatures" && args[2] == "--without-std" {
		for _, s := range reg.HostSignatures() {
			fmt.Println(s)
		}
		return 0
	}
	if args[1] == "--lmcode-signatures" && len(args) == 2 {
		for _, s := range reg.Signatures() {
			fmt.Println(s)
		}
		return 0
	}

	if args[1] == "--sweep" {
		var maxAge int64 = 1
		dryRun := true
		runID, ledgerPath := "", ""
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--delete":
				dryRun = false
			case "--older-than-hours":
				if i+1 >= len(args) {
					fmt.Fprint(os.Stderr, usageText)
					return invoke.Failure("cli", lmcode.NewError(lmcode.CLI_USAGE, usageText))
				}
				val, err := strconv.ParseInt(args[i+1], 10, 64)
				if err != nil || val < 0 {
					fmt.Fprint(os.Stderr, usageText)
					return invoke.Failure("cli", lmcode.NewError(lmcode.CLI_USAGE, usageText))
				}
				maxAge = val
				i++
			case "--run-id":
				if i+1 >= len(args) {
					fmt.Fprint(os.Stderr, usageText)
					return invoke.Failure("cli", lmcode.NewError(lmcode.CLI_USAGE, usageText))
				}
				runID = args[i+1]
				i++
			case "--ledger":
				if i+1 >= len(args) {
					fmt.Fprint(os.Stderr, usageText)
					return invoke.Failure("cli", lmcode.NewError(lmcode.CLI_USAGE, usageText))
				}
				ledgerPath = args[i+1]
				i++
			default:
				fmt.Fprint(os.Stderr, usageText)
				return invoke.Failure("cli", lmcode.NewError(lmcode.CLI_USAGE, usageText))
			}
		}
		if runID != "" || ledgerPath != "" {
			path := ledgerPath
			if path == "" {
				path = safety.JournalPath(safety.DefaultStateDir(), runID)
				if path == "" {
					return invoke.Failure("cli", lmcode.NewError(lmcode.CLI_USAGE, "invalid --run-id"))
				}
			}
			if err := h.LoadRecovery(path); err != nil {
				return invoke.Failure("app", lmcode.NewError(lmcode.APP_FAIL, err.Error()))
			}
			fmt.Printf("recovery: run=%s ledger=%s\n", h.RunID(), h.RecoveryPath())
		}
		if dryRun {
			items := h.SweepDryRun(maxAge)
			fmt.Printf("sweep dry-run (older_than_hours=%d): %d candidate(s)\n", maxAge, len(items))
			for _, it := range items {
				fmt.Println("  " + it)
			}
		} else {
			deleted, absent, retained := h.SweepDeleteDetailed(maxAge)
			fmt.Printf("sweep delete (older_than_hours=%d): deleted %d resource(s), absent %d, retained %d\n", maxAge, deleted, absent, retained)
		}
		return 0
	}

	if invocation {
		contract, err := lmcode.ResultContractFromSchema(schema.EmbeddedSchema)
		if err != nil {
			return invoke.Failure("load", err)
		}
		ctx := lmcode.NewContext()
		ctx.Set("gcp", h)
		ctx.SetDry(dryInvocation)

		var exitOnce sync.Once
		runner := invoke.Runner{
			Results:      results,
			Registry:     reg,
			Schema:       schema.EmbeddedSchema,
			Contract:     contract,
			Context:      ctx,
			Order:        order,
			Dry:          dryInvocation,
			Builder:      builderInvocation,
			RecoveryID:   h.RunID(),
			RecoveryPath: h.RecoveryPath(),
			Hooks: invoke.Hooks{
				BeforeRun: membudget.ResetRun,
				OnRunError: func(runErr error) {
					if strings.Contains(runErr.Error(), membudget.EnvVarName) {
						fmt.Fprintln(os.Stderr, runErr.Error())
					}
				},
				Cleanup:   func() { _ = h.CleanupLedger() },
				AuthError: h.AuthError,
				StartWatchdog: func(onError func(error)) func() {
					return membudget.StartWatchdog(func(watchdogErr error) {
						exitOnce.Do(func() { onError(watchdogErr) })
					})
				},
			},
		}
		return runner.Execute()
	}

	fmt.Fprint(os.Stderr, usageText)
	return invoke.Failure("cli", lmcode.NewError(lmcode.CLI_USAGE, usageText))
}
