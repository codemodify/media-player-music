//go:build ignore

// Exercise the installed toolkit's skip report with actual nested Go tests.
// tools/testenv.sh env GOCACHE=/home/user/.cache/go-build go run -p 2 tools/probes/skips_report.go
// Add -cleanup for logs after Skip, or -failure-status for failed-run reporting.
// Add -context-fallback for unmatched/multiple sites, silent SkipNow and quiet trees.
// Add -multi-package for identical child names in different packages.
// Add -repeat for repeated invocations and parallel CONT/NAME log preservation.
// A failed report assertion prints FAIL and exits 1.
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	cleanup := flag.Bool("cleanup", false, "also require the skip reason when cleanup logs afterward")
	failureStatus := flag.Bool("failure-status", false, "verify the advertised failed-run status and marker")
	contextFallback := flag.Bool("context-fallback", false, "verify full-context, silent-skip and quiet-tree branches")
	multiPackage := flag.Bool("multi-package", false, "verify identical child names retain package-specific reasons")
	repeat := flag.Bool("repeat", false, "verify -count=2 isolation and parallel resume log preservation")
	flag.Parse()
	if err := run(*cleanup, *failureStatus, *contextFallback, *multiPackage, *repeat); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cleanup, failureStatus, contextFallback, multiPackage, repeat bool) error {
	module, err := exec.Command("go", "list", "-m", "-f", "{{.Version}} {{.Dir}}", "github.com/codemodify/uitoolkit").CombinedOutput()
	if err != nil {
		return fmt.Errorf("locate installed toolkit: %w: %s", err, module)
	}
	parts := strings.SplitN(strings.TrimSpace(string(module)), " ", 2)
	if len(parts) != 2 {
		return fmt.Errorf("unexpected module identity: %q", module)
	}
	script, err := os.ReadFile(filepath.Join(parts[1], "tools", "skips.sh"))
	if err != nil {
		return err
	}
	fmt.Printf("toolkit=%s skips.sh sha256=%x\n", parts[0], sha256.Sum256(script))
	dir, err := os.MkdirTemp("", "uitoolkit-skips-report-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := os.Mkdir(filepath.Join(dir, "tools"), 0700); err != nil {
		return err
	}
	fixture := `package skipfixture
import "testing"
func TestParent(t *testing.T) {
 t.Run("plain", func(t *testing.T) { t.Skip("fixture plain child skip reason") })
 t.Run("logged", func(t *testing.T) {
  t.Log("fixture ordinary setup log, not a skip reason")
  t.Skip("fixture actual unavailable capability reason")
 })
}
`
	if cleanup {
		fixture = strings.TrimSuffix(fixture, "}\n") + ` t.Run("cleanup", func(t *testing.T) {
  t.Cleanup(func() { t.Log("fixture cleanup log, not a skip reason") })
  t.Skip("fixture actual cleanup child skip reason")
 })
}
`
	}
	files := map[string]string{
		"go.mod":          "module example.com/skipfixture\n\ngo 1.22.2\n",
		"fixture_test.go": fixture,
		"tools/skips.sh":  string(script),
		// Only the test launcher is replaced; the report script is byte exact.
		"tools/test.sh": "#!/bin/bash\nset -euo pipefail\nexec go test -p 2 \"$@\"\n",
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0700); err != nil {
			return err
		}
	}
	raw := exec.Command("bash", "tools/test.sh", "-v", "-count=1", "./")
	raw.Dir = dir
	rawOutput, err := raw.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fixture tests failed: %w\n%s", err, rawOutput)
	}
	if !strings.Contains(string(rawOutput), "fixture actual unavailable capability reason") ||
		!strings.Contains(string(rawOutput), "--- SKIP: TestParent/logged") {
		return fmt.Errorf("fixture did not produce the actual nested skip:\n%s", rawOutput)
	}
	if cleanup {
		skipAt := strings.Index(string(rawOutput), "fixture actual cleanup child skip reason")
		cleanupAt := strings.Index(string(rawOutput), "fixture cleanup log, not a skip reason")
		if skipAt < 0 || cleanupAt <= skipAt || !strings.Contains(string(rawOutput), "--- SKIP: TestParent/cleanup") {
			return fmt.Errorf("cleanup fixture did not log after its actual Skip:\n%s", rawOutput)
		}
		fmt.Println("PASS: raw fixture includes the actual Skip reason before its cleanup log")
	}
	report := exec.Command("bash", "tools/skips.sh", "-count=1", "./")
	report.Dir = dir
	output, err := report.CombinedOutput()
	if err != nil {
		return fmt.Errorf("report invocation failed: %w\n%s", err, output)
	}
	fmt.Print(string(output))
	plainOK := strings.Contains(string(output), "TestParent/plain") && strings.Contains(string(output), "fixture plain child skip reason")
	loggedFound := strings.Contains(string(output), "TestParent/logged")
	fmt.Printf("nested child detection: plain=%t logged=%t\n", plainOK, loggedFound)
	if !plainOK || !loggedFound {
		return fmt.Errorf("FAIL: nested child detection control")
	}
	fmt.Println("PASS: nested child detection control")
	if !strings.Contains(string(output), "fixture actual unavailable capability reason") {
		return fmt.Errorf("FAIL: expected logged child's actual t.Skip reason, report retained the earlier t.Log instead")
	}
	fmt.Println("PASS: report includes the actual skip reason after an earlier setup log")
	if cleanup {
		if !strings.Contains(string(output), "TestParent/cleanup") || !strings.Contains(string(output), "fixture actual cleanup child skip reason") {
			return fmt.Errorf("FAIL: expected cleanup child's actual t.Skip reason, report retained the later cleanup t.Log instead")
		}
		fmt.Println("PASS: report includes the actual skip reason before a later cleanup log")
	}
	if failureStatus {
		if err := checkFailureStatus(dir); err != nil {
			return err
		}
	}
	if contextFallback {
		if err := checkContextFallback(dir); err != nil {
			return err
		}
	}
	if multiPackage {
		if err := checkMultiplePackages(dir); err != nil {
			return err
		}
	}
	if repeat {
		if err := checkRepeatedInvocations(dir); err != nil {
			return err
		}
	}
	return nil
}

func checkRepeatedInvocations(dir string) error {
	fixture := `package skipfixture
import "testing"
var attempts int
var resumeReady = make(chan struct{})
var cursorLogged = make(chan struct{})
func TestRepeated(t *testing.T) {
 attempts++
 if attempts == 1 { t.Skip("only the first attempt has a reason") }
 t.SkipNow()
}
func TestParallelResume(t *testing.T) {
 t.Log("parallel pre-pause context")
 t.Parallel()
 close(resumeReady)
 <-cursorLogged
 t.Log("parallel post-resume context")
 t.SkipNow()
}
func TestParallelCursor(t *testing.T) {
 t.Parallel()
 <-resumeReady
 t.Log("parallel cursor marker")
 close(cursorLogged)
}
`
	if err := os.WriteFile(filepath.Join(dir, "repeat_test.go"), []byte(fixture), 0600); err != nil {
		return err
	}
	run := func(script string, args ...string) (string, error) {
		args = append([]string{script, "-v"}, args...)
		cmd := exec.Command("bash", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	parallelArgs := []string{"-count=1", "-parallel=2", "-timeout=10s", "-run", "^TestParallel", "./"}
	raw, err := run("tools/test.sh", parallelArgs...)
	if err != nil {
		return fmt.Errorf("parallel fixture failed: %w\n%s", err, raw)
	}
	continued, named := false, false
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "===" && fields[2] == "TestParallelResume" {
			continued = continued || fields[1] == "CONT"
			named = named || fields[1] == "NAME"
		}
	}
	if !continued || !named || !strings.Contains(raw, "parallel pre-pause context") || !strings.Contains(raw, "parallel post-resume context") {
		return fmt.Errorf("FAIL: fixture did not exercise actual CONT/NAME and both logs: CONT=%t NAME=%t\n%s", continued, named, raw)
	}
	fmt.Print(raw)
	parallel, err := run("tools/skips.sh", parallelArgs...)
	if err != nil {
		return fmt.Errorf("parallel report failed: %w\n%s", err, parallel)
	}
	fmt.Print(parallel)
	if !strings.Contains(parallel, "TestParallelResume") || !strings.Contains(parallel, "parallel pre-pause context") || !strings.Contains(parallel, "parallel post-resume context") || !strings.Contains(parallel, "1 skipped") || strings.Contains(parallel, "parallel cursor marker") {
		return fmt.Errorf("FAIL: CONT/NAME lost existing logs or leaked the other parallel test's log")
	}
	fmt.Println("PASS: actual parallel CONT/NAME preserve both logs and test attribution")
	repeatArgs := []string{"-count=2", "-timeout=10s", "-run", "^TestRepeated$", "./"}
	raw, err = run("tools/test.sh", repeatArgs...)
	if err != nil {
		return fmt.Errorf("repeated fixture failed: %w\n%s", err, raw)
	}
	if strings.Count(raw, "=== RUN   TestRepeated") != 2 || strings.Count(raw, "only the first attempt has a reason") != 1 || strings.Count(raw, "--- SKIP: TestRepeated") != 2 {
		return fmt.Errorf("FAIL: fixture did not execute two attempts with only the first saying anything:\n%s", raw)
	}
	fmt.Print(raw)
	fmt.Println("PASS: raw -count=2 fixture has a reason only on its first invocation")
	report, err := run("tools/skips.sh", repeatArgs...)
	if err != nil {
		return fmt.Errorf("repeated report failed: %w\n%s", err, report)
	}
	fmt.Print(report)
	var rows []string
	for _, line := range strings.Split(report, "\n") {
		if strings.HasPrefix(line, "TestRepeated ") {
			rows = append(rows, line)
		}
	}
	if len(rows) != 2 || !strings.Contains(report, "2 skipped") {
		return fmt.Errorf("FAIL: expected two flat repeated-invocation rows/count, got %d rows", len(rows))
	}
	firstOK := strings.Contains(rows[0], "only the first attempt has a reason")
	secondOK := strings.Contains(rows[1], "(said nothing)") && !strings.Contains(rows[1], "only the first attempt has a reason")
	fmt.Printf("repeated invocation isolation: first=%t secondSilent=%t\n", firstOK, secondOK)
	if !firstOK || !secondOK {
		return fmt.Errorf("FAIL: second RUN borrowed the first invocation's log")
	}
	fmt.Println("PASS: repeated RUN clears invocation logs without clearing CONT/NAME")
	return nil
}

func checkMultiplePackages(dir string) error {
	const firstReason = "capability missing only in first package"
	fixtures := map[string]string{
		"first/first_test.go": `package first
import "testing"
func TestShared(t *testing.T) {
 t.Run("child", func(t *testing.T) { t.Skip("capability missing only in first package") })
}
`,
		"second/second_test.go": `package second
import "testing"
func TestShared(t *testing.T) {
 t.Run("child", func(t *testing.T) { t.SkipNow() })
}
`,
	}
	for name, body := range fixtures {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			return err
		}
	}
	run := func(script string, packages ...string) (string, error) {
		args := append([]string{script, "-v", "-count=1"}, packages...)
		cmd := exec.Command("bash", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	const firstPackage = "example.com/skipfixture/first"
	const secondPackage = "example.com/skipfixture/second"
	raw, err := run("tools/test.sh", "./first/", "./second/")
	if err != nil {
		return fmt.Errorf("multiple-package fixture failed: %w\n%s", err, raw)
	}
	block := ""
	firstRaw, secondRaw := false, false
	for _, line := range strings.Split(raw, "\n") {
		block += line + "\n"
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "ok" {
			continue
		}
		if fields[1] == firstPackage {
			firstRaw = strings.Contains(block, firstReason) && strings.Contains(block, "--- SKIP: TestShared/child")
		}
		if fields[1] == secondPackage {
			secondRaw = !strings.Contains(block, firstReason) && strings.Contains(block, "--- SKIP: TestShared/child")
		}
		block = ""
	}
	if !firstRaw || !secondRaw {
		return fmt.Errorf("FAIL: real package blocks did not contain a reason only in the first: first=%t second=%t\n%s", firstRaw, secondRaw, raw)
	}
	fmt.Println("PASS: raw Go output scopes the reason to the first package; second child is silent")
	single, err := run("tools/skips.sh", "./first/")
	if err != nil || !strings.Contains(single, firstReason) || !strings.Contains(single, "1 skipped") || strings.Contains(single, firstPackage) {
		return fmt.Errorf("FAIL: single-package report changed its reason/count/heading contract: %v\n%s", err, single)
	}
	fmt.Println("PASS: single reported package retains a flat test row without a package heading")
	report, err := run("tools/skips.sh", "./first/", "./second/")
	if err != nil {
		return fmt.Errorf("multiple-package report failed: %w\n%s", err, report)
	}
	fmt.Print(report)
	type row struct{ pkg, text string }
	var rows []row
	pkg := ""
	for _, line := range strings.Split(report, "\n") {
		if line == firstPackage || line == secondPackage {
			pkg = line
		}
		if strings.HasPrefix(line, "TestShared/child ") {
			rows = append(rows, row{pkg, line})
		}
	}
	if len(rows) != 2 || !strings.Contains(report, "2 skipped") {
		return fmt.Errorf("FAIL: expected two flat shared-name child rows and count two; got %d rows", len(rows))
	}
	firstOK := rows[0].pkg == firstPackage && strings.Contains(rows[0].text, firstReason)
	secondOK := rows[1].pkg == secondPackage && strings.Contains(rows[1].text, "(said nothing)") && !strings.Contains(rows[1].text, firstReason)
	fmt.Printf("package-scoped reasons: first=%t secondSilent=%t\n", firstOK, secondOK)
	if !firstOK || !secondOK {
		return fmt.Errorf("FAIL: identical child names did not retain their own package's reason/silence")
	}
	fmt.Println("PASS: multiple-package reason isolation, headings and flat test rows")
	return nil
}

func checkContextFallback(dir string) error {
	// Preserve the earlier assertions, then use a separate fixture selection.
	if err := os.Remove(filepath.Join(dir, "failure_test.go")); err != nil && !os.IsNotExist(err) {
		return err
	}
	fixtures := map[string]string{
		"helper.go": `package skipfixture
import "testing"
func unmatchedSkip(t *testing.T) {
 t.Log("context helper setup")
 t.Cleanup(func() { t.Log("context helper cleanup") })
 t.Skip("context helper actual reason")
}
`,
		"fixture_test.go": `package skipfixture
import "testing"
func TestContext(t *testing.T) {
 t.Run("unmatched", func(t *testing.T) { unmatchedSkip(t) })
 t.Run("multiple", func(t *testing.T) {
  t.Log("context same-line setup"); t.Skip("context same-line actual reason")
 })
 t.Run("silent", func(t *testing.T) { t.SkipNow() })
}
`,
	}
	for name, body := range fixtures {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			return err
		}
	}
	run := func(script string) (string, error) {
		cmd := exec.Command("bash", script, "-v", "-count=1", "./")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	wantLogs := []string{
		"context helper setup", "context helper actual reason", "context helper cleanup",
		"context same-line setup", "context same-line actual reason",
	}
	raw, err := run("tools/test.sh")
	if err != nil {
		return fmt.Errorf("context fixture failed: %w\n%s", err, raw)
	}
	for _, want := range wantLogs {
		if !strings.Contains(raw, want) {
			return fmt.Errorf("context fixture lacks %q:\n%s", want, raw)
		}
	}
	report, err := run("tools/skips.sh")
	if err != nil {
		return fmt.Errorf("context report failed: %w\n%s", err, report)
	}
	fmt.Print(report)
	for _, want := range append(wantLogs, "TestContext/unmatched", "TestContext/multiple", "3 skipped", "2 of them logged lines this could not trace to a Skip call") {
		if !strings.Contains(report, want) {
			return fmt.Errorf("FAIL: full-context report omits %q", want)
		}
	}
	silentOK := false
	for _, line := range strings.Split(report, "\n") {
		if strings.Contains(line, "TestContext/silent") && strings.Contains(line, "(said nothing)") {
			silentOK = true
		}
	}
	if !silentOK {
		return fmt.Errorf("FAIL: silent SkipNow did not report that it said nothing")
	}
	fmt.Println("PASS: unmatched/multiple-site full context and silent SkipNow")
	for name := range fixtures {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	quiet := "package skipfixture\nimport \"testing\"\nfunc TestQuiet(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(dir, "quiet_test.go"), []byte(quiet), 0600); err != nil {
		return err
	}
	report, err = run("tools/skips.sh")
	fmt.Print(report)
	if err != nil || !strings.Contains(report, "0 skipped") {
		return fmt.Errorf("FAIL: no-Skip source tree report: %v\n%s", err, report)
	}
	fmt.Println("PASS: quiet source tree reports zero skips with exit 0")
	// This tree still contains no Skip call when the failure fixture is added.
	if err := checkFailureStatus(dir); err != nil {
		return err
	}
	fmt.Println("PASS: quiet source tree also preserves failed-run status")
	return nil
}

func checkFailureStatus(dir string) error {
	fixture := "package skipfixture\nimport \"testing\"\nfunc TestInvocationFailure(t *testing.T) { t.Fatal(\"fixture actual assertion failure\") }\n"
	if err := os.WriteFile(filepath.Join(dir, "failure_test.go"), []byte(fixture), 0600); err != nil {
		return err
	}
	run := func(script string) (string, int, error) {
		cmd := exec.Command("bash", script, "-v", "-count=1", "-run", "^TestInvocationFailure$", "./")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if ee, ok := err.(*exec.ExitError); ok {
			return string(out), ee.ExitCode(), nil
		}
		return string(out), 0, err
	}
	raw, rawStatus, err := run("tools/test.sh")
	if err != nil {
		return err
	}
	if rawStatus == 0 || !strings.Contains(raw, "fixture actual assertion failure") {
		return fmt.Errorf("FAIL: failed-test fixture control exited %d:\n%s", rawStatus, raw)
	}
	report, reportStatus, err := run("tools/skips.sh")
	if err != nil {
		return err
	}
	fmt.Print(report)
	marked := strings.Contains(report, "the test run itself failed")
	fmt.Printf("failed invocation: rawStatus=%d reportStatus=%d failureMarker=%t\n", rawStatus, reportStatus, marked)
	if reportStatus != rawStatus || !marked {
		return fmt.Errorf("FAIL: reporter did not preserve the failed-run status and diagnostic")
	}
	fmt.Println("PASS: advertised failed-run status and marker")
	return nil
}
