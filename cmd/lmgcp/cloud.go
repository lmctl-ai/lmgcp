// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"strings"

	"github.com/lmctl-ai/lmcloud/hostkit/cloudhost"
	"github.com/lmctl-ai/lmcloud/hostkit/invoke"
	"github.com/lmctl-ai/lmcloud/schema"

	"github.com/lmctl-ai/lmgcp/internal/gcp"
	"github.com/lmctl-ai/lmgcp/internal/membudget"
)

// cloudRealMain serves the shared lmcloud schema on hostkit. The legacy lmgcp
// surface is untouched; only `lmgcp cloud ...` uses this path.
func cloudRealMain(args []string) int {
	v := strings.TrimSpace(rawVersion)
	if v == "" {
		v = "0.1.0"
	}
	p := cloudhost.Program{
		Name:    "lmgcp cloud",
		Version: v,
		Schema:  schema.Bytes,
		Out:     os.Stdout,
		Init: func() error {
			_, err := membudget.InitFromEnv()
			return err
		},
		Hooks: invoke.Hooks{
			BeforeRun:     membudget.ResetRun,
			StartWatchdog: membudget.StartWatchdog,
		},
		NewConfig: func(dry bool) (cloudhost.Config, error) {
			a := gcp.NewAdapter()
			return cloudhost.Config{
				Provider:  "gcp",
				Store:     a,
				Queue:     a,
				Endpoints: gcp.Endpoints(),
			}, nil
		},
	}
	return p.Main(args)
}
