package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"ok-folio/internal/preflight"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()
	_ = w.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy: %v", err)
	}
	return buf.String()
}

func TestPrintResultRendersStatusAndEvidence(t *testing.T) {
	out := captureStdout(t, func() {
		printResult(preflight.Result{
			ID:       "example-check",
			Status:   preflight.StatusPass,
			Summary:  "everything is fine",
			Evidence: []string{"file.yaml: no legacy var"},
		})
	})
	if !strings.Contains(out, "[PASS] example-check: everything is fine") {
		t.Errorf("missing status line, got:\n%s", out)
	}
	if !strings.Contains(out, "- file.yaml: no legacy var") {
		t.Errorf("missing evidence line, got:\n%s", out)
	}
}

func TestPrintSummaryReadyWithPending(t *testing.T) {
	report := preflight.Report{Results: []preflight.Result{
		{ID: "a", Status: preflight.StatusPass},
		{ID: "b", Status: preflight.StatusPending},
	}}
	out := captureStdout(t, func() { printSummary(report) })
	if !strings.Contains(out, "1 PASS, 0 WARN, 0 FAIL, 1 PENDING") {
		t.Errorf("bad counts, got:\n%s", out)
	}
	if !strings.Contains(out, "READY-WITH-PENDING") {
		t.Errorf("expected READY-WITH-PENDING, got:\n%s", out)
	}
}

func TestPrintSummaryNotReadyOnFail(t *testing.T) {
	report := preflight.Report{Results: []preflight.Result{
		{ID: "a", Status: preflight.StatusFail},
	}}
	out := captureStdout(t, func() { printSummary(report) })
	if !strings.Contains(out, "NOT READY") {
		t.Errorf("expected NOT READY, got:\n%s", out)
	}
}

func TestPrintSummaryReady(t *testing.T) {
	report := preflight.Report{Results: []preflight.Result{
		{ID: "a", Status: preflight.StatusPass},
		{ID: "b", Status: preflight.StatusPass},
	}}
	out := captureStdout(t, func() { printSummary(report) })
	if !strings.Contains(out, "Result: READY") || strings.Contains(out, "READY-WITH-PENDING") {
		t.Errorf("expected plain READY, got:\n%s", out)
	}
}
