package observability

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInitOTelNoEndpoint(t *testing.T) {
	ctx := context.Background()
	providers, err := InitOTel(ctx, OTelConfig{
		ServiceName: "dmgn-test",
	})
	if err != nil {
		t.Fatalf("InitOTel failed: %v", err)
	}
	if providers == nil {
		t.Fatal("expected non-nil providers")
	}
	if providers.TracerProvider == nil {
		t.Fatal("expected non-nil tracer provider")
	}
	if providers.MeterProvider == nil {
		t.Fatal("expected non-nil meter provider")
	}

	if err := providers.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
}

func TestDMGNMetrics(t *testing.T) {
	ctx := context.Background()
	providers, err := InitOTel(ctx, OTelConfig{ServiceName: "dmgn-test"})
	if err != nil {
		t.Fatalf("InitOTel failed: %v", err)
	}
	defer providers.Shutdown(ctx)

	metrics, err := NewDMGNMetrics()
	if err != nil {
		t.Fatalf("NewDMGNMetrics failed: %v", err)
	}

	// Record values — should not panic
	metrics.MemoryCount.Add(ctx, 1)
	metrics.QueryLatency.Record(ctx, 42.5)
	metrics.PeerCount.Add(ctx, 3)
	metrics.SyncEvents.Add(ctx, 1)
	metrics.GossipMessages.Add(ctx, 5)
	metrics.VectorIndexSize.Add(ctx, 100)
}

func TestInitLogging(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	logOut := InitLogging(LogConfig{
		Level:      "debug",
		LogDir:     logDir,
		MaxSizeMB:  1,
		MaxBackups: 2,
		Stderr:     false,
	})
	if logOut == nil {
		t.Fatal("expected non-nil log output")
	}
	if logOut.Logger == nil {
		t.Fatal("expected non-nil logger")
	}

	logOut.Logger.Info("test log entry", "key", "value")

	// Close file handles before TempDir cleanup
	logOut.Close()

	// Verify log file was created
	logFile := filepath.Join(logDir, "dmgn.log")
	info, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("log file is empty")
	}
}

func TestLogLevelParsing(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"debug", "DEBUG"},
		{"info", "INFO"},
		{"warn", "WARN"},
		{"warning", "WARN"},
		{"error", "ERROR"},
		{"unknown", "INFO"},
		{"", "INFO"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := LogLevelFromString(tt.input)
			if got.String() != tt.want {
				t.Fatalf("LogLevelFromString(%q) = %s, want %s", tt.input, got.String(), tt.want)
			}
		})
	}
}

func TestShutdown(t *testing.T) {
	ctx := context.Background()
	providers, err := InitOTel(ctx, OTelConfig{ServiceName: "dmgn-test"})
	if err != nil {
		t.Fatalf("InitOTel failed: %v", err)
	}

	// Double shutdown should not error
	if err := providers.Shutdown(ctx); err != nil {
		t.Fatalf("first shutdown failed: %v", err)
	}
}

func TestInitLoggingStderrOnly(t *testing.T) {
	logOut := InitLogging(LogConfig{
		Level:  "info",
		Stderr: true,
	})
	if logOut == nil || logOut.Logger == nil {
		t.Fatal("expected non-nil logger")
	}
	logOut.Close()
}
