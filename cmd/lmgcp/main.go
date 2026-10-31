// SPDX-License-Identifier: MIT

package main

import (
	_ "embed"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/lmctl-ai/lmcode-go"
	"github.com/lmctl-ai/lmgcp/internal/host"
	"github.com/lmctl-ai/lmgcp/internal/membudget"
	"github.com/lmctl-ai/lmgcp/internal/safety"
	"github.com/lmctl-ai/lmgcp/schema"
)

//go:embed VERSION
var rawVersion string

//go:embed THIRD_PARTY_NOTICES
var embeddedNotices string

const usageText = `usage: lmgcp --version | --help | --notices | --lmcode-schema [--json] | --lmcode-signatures [--without-std] | --lmcode [--dry-run] FILE [FILE ...] | --sweep [--older-than-hours N] [--delete]

Google Cloud Go SDK host for lmscript (Apache-2.0 SDK on lmcode-go).

Options:
  --version                  Print version (lmgcp X.Y.Z)
  --help, -h                 Print this usage and safety overview
  --notices                  Print embedded third-party license notices
  --lmcode-schema [--json]      Print human-readable schema summary or raw JSON schema
  --lmcode-signatures           Print sorted canonical DSL signatures
  --lmcode [--dry-run] FILE...  Load, verify, and execute compiled .dslb bytecode files
  --sweep                    List (dry-run default) or delete (--delete) lmgcp_ leftovers

Environment & Resource Limits:
  LMGCP_MAX_MEMORY_MB        Total memory budget in whole MiB (default: 4096; invalid = exit 2)
  LMGCP_LIVE                 Set to 1 to use Google Application Default Credentials (default: 0, in-process fake GCP)
  LMGCP_ENDPOINT            Loopback HTTP emulator origin (mutually exclusive with live mode)
  LMGCP_ACCESS_TOKEN        Optional dummy bearer token for the emulator
  LMGCP_PROJECT              Target GCP project ID (masked in all outputs)
  LMGCP_REGION               Default GCP region (default: us-central1)
  LMGCP_ZONE                 Default GCP zone (default: us-central1-a)

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
	if _, err := membudget.InitFromEnv(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 2
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
	if len(args) < 2 || (args[1] != "--lmcode-schema" && args[1] != "--lmcode-signatures" && args[1] != "--lmcode" && args[1] != "--sweep") {
		fmt.Fprint(os.Stderr, usageText)
		return 2
	}
	h, err := host.NewHost()
	if err != nil {
		return lmcode.ReportAppFailure("failed to initialize GCP host: " + safety.RedactSecrets(err.Error()))
	}
	defer h.Close()

	reg := lmcode.NewRegistry()
	if err := host.Register(reg, h); err != nil {
		return lmcode.ReportAppFailure("failed to register host: " + err.Error())
	}

	if len(args) >= 2 && args[1] == "--lmcode-schema" {
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
		return 2
	}

	if len(args) == 3 && args[1] == "--lmcode-signatures" && args[2] == "--without-std" {
		for _, s := range reg.HostSignatures() {
			fmt.Println(s)
		}
		return 0
	}
	if len(args) == 2 && args[1] == "--lmcode-signatures" {
		for _, s := range reg.Signatures() {
			fmt.Println(s)
		}
		return 0
	}

	if len(args) >= 2 && args[1] == "--sweep" {
		var maxAge int64 = 1
		dryRun := true
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--delete":
				dryRun = false
			case "--older-than-hours":
				if i+1 >= len(args) {
					fmt.Fprint(os.Stderr, usageText)
					return 2
				}
				val, err := strconv.ParseInt(args[i+1], 10, 64)
				if err != nil || val < 0 {
					fmt.Fprint(os.Stderr, usageText)
					return 2
				}
				maxAge = val
				i++
			default:
				fmt.Fprint(os.Stderr, usageText)
				return 2
			}
		}
		if dryRun {
			items := h.SweepDryRun(maxAge)
			fmt.Printf("sweep dry-run (older_than_hours=%d): %d candidate(s)\n", maxAge, len(items))
			for _, it := range items {
				fmt.Println("  " + it)
			}
		} else {
			del := h.SweepDelete(maxAge)
			fmt.Printf("sweep delete (older_than_hours=%d): deleted %d resource(s)\n", maxAge, del)
		}
		return 0
	}

	if len(args) >= 3 && args[1] == "--lmcode" {
		dryRun := false
		startIdx := 2
		if args[2] == "--dry-run" {
			dryRun = true
			startIdx = 3
		}
		if startIdx >= len(args) {
			fmt.Fprint(os.Stderr, usageText)
			return 2
		}
		for i := startIdx; i < len(args); i++ {
			if args[i] == "" || strings.HasPrefix(args[i], "--") {
				fmt.Fprint(os.Stderr, usageText)
				return 2
			}
		}

		ctx := lmcode.NewContext()
		ctx.Set("gcp", h)

		for i := startIdx; i < len(args); i++ {
			codeBytes, err := os.ReadFile(args[i])
			if err != nil {
				return lmcode.ReportError("load", lmcode.NewError(lmcode.LOAD_BAD_BYTECODE, err.Error()))
			}
			prog, err := lmcode.Load(codeBytes, schema.EmbeddedSchema, reg)
			if err != nil {
				return lmcode.ReportError("load", err)
			}
			if dryRun {
				lmcode.ReportOK()
				continue
			}

			membudget.ResetRun()
			var exitOnce sync.Once
			stopWatchdog := membudget.StartWatchdog(func(wErr error) {
				exitOnce.Do(func() {
					_ = h.CleanupLedger()
					fmt.Fprintln(os.Stderr, wErr.Error())
					code := lmcode.ReportError("run", wErr)
					os.Exit(code)
				})
			})
			runErr := prog.Run(ctx, reg)
			stopWatchdog()
			if h.AuthError() != "" {
				return lmcode.ReportAppFailure(h.AuthError())
			}
			if runErr != nil {
				_ = h.CleanupLedger()
				if strings.Contains(runErr.Error(), membudget.EnvVarName) {
					fmt.Fprintln(os.Stderr, runErr.Error())
				}
				return lmcode.ReportError("run", runErr)
			}
			lmcode.ReportOK()
		}
		return 0
	}

	fmt.Fprint(os.Stderr, usageText)
	return 2
}
