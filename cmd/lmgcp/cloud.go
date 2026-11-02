// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/lmctl-ai/lmcode-go"
	"github.com/lmctl-ai/lmgcp/internal/cloudhost"
	"github.com/lmctl-ai/lmgcp/internal/invoke"
	"github.com/lmctl-ai/lmgcp/internal/membudget"
	"github.com/lmctl-ai/lmgcp/schema"
)

const cloudUsage = `usage: lmgcp cloud --version | --help | --notices | --lmcode-schema [--json] | --lmcode-signatures [--without-std] | [--lmcode|--first|--last] FILE [FILE ...]

Shared lmcloud schema host for Google Cloud Storage + Pub/Sub (sim/fake only).
`

func cloudRealMain(args []string) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(cloudUsage)
		return 0
	}
	if len(args) == 1 && args[0] == "--version" {
		v := strings.TrimSpace(rawVersion)
		if v == "" {
			v = "0.1.0"
		}
		fmt.Printf("lmgcp cloud %s\n", v)
		return 0
	}
	if len(args) == 1 && args[0] == "--notices" {
		fmt.Print(embeddedNotices)
		return 0
	}
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, cloudUsage)
		return invoke.Failure("cli", lmcode.NewError(lmcode.CLI_USAGE, cloudUsage))
	}

	h := cloudhost.New()
	reg := lmcode.NewRegistry()
	if err := cloudhost.Register(reg, h); err != nil {
		return invoke.Failure("app", lmcode.NewError(lmcode.APP_FAIL, err.Error()))
	}

	switch args[0] {
	case "--lmcode-schema":
		if len(args) == 1 {
			for _, sig := range reg.Signatures() {
				fmt.Println(sig)
			}
			return 0
		}
		if len(args) == 2 && args[1] == "--json" {
			_, err := os.Stdout.Write(schema.EmbeddedLMCloudSchema)
			if err != nil {
				return 4
			}
			return 0
		}
		return invoke.Failure("cli", lmcode.NewError(lmcode.CLI_USAGE, cloudUsage))
	case "--lmcode-signatures":
		if len(args) == 2 && args[1] == "--without-std" {
			for _, sig := range reg.HostSignatures() {
				fmt.Println(sig)
			}
			return 0
		}
		if len(args) == 1 {
			for _, sig := range reg.Signatures() {
				fmt.Println(sig)
			}
			return 0
		}
		return invoke.Failure("cli", lmcode.NewError(lmcode.CLI_USAGE, cloudUsage))
	}

	invocation := args[0] == "--lmcode" || args[0] == "--first" || args[0] == "--last"
	if !invocation {
		return invoke.Failure("cli", lmcode.NewError(lmcode.CLI_USAGE, cloudUsage))
	}
	type part struct {
		file            string
		priority, index int
	}
	var parts []part
	var files []string
	add := func(file string, priority int) bool {
		if file == "" || strings.HasPrefix(file, "--") {
			return false
		}
		parts = append(parts, part{file, priority, len(parts)})
		files = append(files, file)
		return true
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--lmcode", "--first", "--last":
			priority := 1
			if args[i] == "--first" {
				priority = 0
			} else if args[i] == "--last" {
				priority = 2
			}
			if i+1 >= len(args) || !add(args[i+1], priority) {
				return invoke.Failure("cli", lmcode.NewError(lmcode.CLI_USAGE, cloudUsage))
			}
			i++
		default:
			if !add(args[i], 1) {
				return invoke.Failure("cli", lmcode.NewError(lmcode.CLI_USAGE, cloudUsage))
			}
		}
	}
	if len(parts) == 0 {
		return invoke.Failure("cli", lmcode.NewError(lmcode.CLI_USAGE, cloudUsage))
	}
	results, err := lmcode.InvocationResultsFromCommand(files)
	if err != nil {
		return invoke.Failure("cli", err)
	}
	order := make([]int, len(parts))
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
	contract, err := lmcode.ResultContractFromSchema(schema.EmbeddedLMCloudSchema)
	if err != nil {
		return invoke.Failure("load", err)
	}
	ctx := lmcode.NewContext()
	ctx.Set("cloud", h)
	ctx.SetDry(results.Dry())
	var once sync.Once
	runner := invoke.Runner{
		Results: results, Registry: reg, Schema: schema.EmbeddedLMCloudSchema, Contract: contract, Context: ctx,
		Order: order, Dry: results.Dry(),
		Hooks: invoke.Hooks{
			BeforeRun: membudget.ResetRun,
			StartWatchdog: func(onError func(error)) func() {
				return membudget.StartWatchdog(func(e error) { once.Do(func() { onError(e) }) })
			},
			OnRunError: func(err error) { fmt.Fprintln(os.Stderr, err.Error()) },
		},
	}
	return runner.Execute()
}
