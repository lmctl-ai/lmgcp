// SPDX-License-Identifier: MIT

package host

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmctl-ai/lmcode-go"
)

func findLmscriptBin(t *testing.T) string {
	t.Helper()
	if bin := os.Getenv("LMSCRIPT_BIN"); bin != "" {
		return bin
	}
	if bin, err := exec.LookPath("lmscript"); err == nil {
		return bin
	}
	for _, candidate := range []string{
		"/home/ubuntu/.nvm/versions/node/v24.22.0/bin/lmscript",
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	t.Skip("lmscript not on PATH, skipping std REST/CLI test")
	return ""
}

func TestStdRestAndCliInterfaces(t *testing.T) {
	h, err := NewHost(false)
	if err != nil {
		t.Fatalf("NewHost failed: %v", err)
	}
	defer h.Close()

	reg := lmcode.NewRegistry()
	if err := Register(reg, h); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if got := len(reg.StdSignatures()); got != 226 {
		t.Fatalf("expected 226 std signatures in lmcode_std_v3, got %d", got)
	}

	lmscriptBin := findLmscriptBin(t)
	tmp := t.TempDir()
	hostBin := filepath.Join(tmp, "lmgcp")
	if out, err := exec.Command("go", "build", "-o", hostBin, "../../cmd/lmgcp").CombinedOutput(); err != nil {
		t.Fatalf("go build lmgcp failed: %v\n%s", err, out)
	}
	schemaPath, err := filepath.Abs(filepath.Join("..", "..", "schema", "lmgcp.dsl.json"))
	if err != nil {
		t.Fatal(err)
	}

	lms := `
s = load_schema(schema="` + schemaPath + `");
p = _lmcode_ s;
  @"/buckets/:name/:key"
  fn get_obj(x: RestExchange) -> void implements RestGet {
    let body = gcp.storage_read_object(x.param("name"), x.param("key"));
    x.send(x.get("cli_format") + ":" + body);
  }

  @"--format"
  fn opt_format(x: RestExchange) -> void implements CliOption {
    x.set("cli_format", x.query("format"));
  }

  @"inspect"
  fn cmd_inspect(x: RestExchange) -> void implements CliCommand {
    x.send("gcp-cli:" + x.get("cli_format") + ":" + x.body());
  }

  rest_config("prefix", "/v1");
  gcp.storage_create_bucket("lmgcp-rest-bkt", "us-central1", "STANDARD");
  gcp.storage_put_object("lmgcp-rest-bkt", "hello.txt", "gcp-rest-ok", "text/plain");

  let cli_out = rest_cli(str_split("--format json inspect -- ready", " "));
  let get_out = rest_cli(str_split("/v1/buckets/lmgcp-rest-bkt/hello.txt", " "));
  gcp.storage_delete_object("lmgcp-rest-bkt", "hello.txt");
  gcp.storage_delete_bucket("lmgcp-rest-bkt");

  lmcode_return_put("cli_out", cli_out);
  lmcode_return_put("get_out", get_out);
end _lmcode_
r = run_lmcode(p, {command: ["` + hostBin + `", "--lmcode", "$DSLFILE"]});
assert(r.ok, "expected lmgcp std rest/cli run to succeed");
assert(r.result.cli_out == "gcp-cli:json:ready", "unexpected cli_out: " + r.result.cli_out);
assert(r.result.get_out == "json:gcp-rest-ok", "unexpected get_out: " + r.result.get_out);
println("LMGCP_STD_REST_CLI_OK");
`
	lmsPath := filepath.Join(tmp, "rest_cli.lms")
	if err := os.WriteFile(lmsPath, []byte(lms), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(lmscriptBin, lmsPath)
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "LMGCP_STD_REST_CLI_OK") {
		t.Fatalf("lmgcp std rest/cli failed: %v\n%s", err, out)
	}
}
