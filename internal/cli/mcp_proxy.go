package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nnlgsakib/dmgn/internal/config"
	"github.com/nnlgsakib/dmgn/internal/daemon"
)

// MCPCmd returns the cobra command for `dmgn mcp`.
// This is a thin stdio↔TCP proxy that AI tools spawn to communicate with
// the running DMGN daemon's MCP server.
func MCPCmd() *cobra.Command {
	var dataDir string
	var portFlag int
	var verbose bool

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Connect to DMGN daemon via MCP (stdio)",
		Long: `Bridges stdin/stdout to the running DMGN daemon's MCP server.

AI tools (Claude Desktop, Cline, Windsurf, etc.) should be configured to run:
  dmgn mcp --port <port>

Use --port to specify the daemon's MCP IPC port directly.
Without --port, the port is read from the daemon's port file in the data directory.
Use --verbose to see all JSON-RPC messages flowing through the proxy.

The daemon must be running (start with: dmgn start).`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			var port string

			if cmd.Flags().Changed("port") && portFlag > 0 {
				// Use the explicitly provided port
				port = fmt.Sprintf("%d", portFlag)
			} else {
				// Fall back to port file lookup
				cfg, err := config.Load(dataDir)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: failed to load config: %v\n", err)
					os.Exit(1)
				}

				_, running := daemon.CheckDaemonRunning(cfg.PIDFile())
				if !running {
					fmt.Fprintf(os.Stderr, "DMGN daemon is not running. Start it with: dmgn start\n")
					os.Exit(1)
				}

				portData, err := os.ReadFile(cfg.PortFile())
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: cannot read MCP port file: %v\n", err)
					fmt.Fprintf(os.Stderr, "Hint: use --port <port> to specify the port directly.\n")
					os.Exit(1)
				}
				port = strings.TrimSpace(string(portData))
			}

			if verbose {
				fmt.Fprintf(os.Stderr, "[mcp] connecting to daemon on 127.0.0.1:%s\n", port)
			}

			// Connect to daemon's MCP IPC
			conn, err := net.Dial("tcp", "127.0.0.1:"+port)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: cannot connect to daemon MCP (port %s): %v\n", port, err)
				os.Exit(1)
			}
			defer conn.Close()

			if verbose {
				fmt.Fprintf(os.Stderr, "[mcp] connected to daemon MCP on port %s\n", port)
			}

			errChan := make(chan error, 2)

			if verbose {
				// Verbose mode: intercept and log each JSON-RPC line
				go func() {
					errChan <- proxyLines(os.Stdin, conn, "client→daemon", os.Stderr)
				}()
				go func() {
					errChan <- proxyLines(conn, os.Stdout, "daemon→client", os.Stderr)
				}()
			} else {
				// Silent mode: raw byte copy
				go func() {
					_, err := io.Copy(conn, os.Stdin)
					errChan <- err
				}()
				go func() {
					_, err := io.Copy(os.Stdout, conn)
					errChan <- err
				}()
			}

			<-errChan
			if verbose {
				fmt.Fprintf(os.Stderr, "[mcp] connection closed\n")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory")
	cmd.Flags().IntVar(&portFlag, "port", 0, "MCP IPC port (skip port file lookup)")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Log all MCP JSON-RPC messages to stderr")

	return cmd
}

// proxyLines reads newline-delimited JSON-RPC messages from src, logs them
// to logDst with a direction label, and writes them through to dst.
func proxyLines(src io.Reader, dst io.Writer, direction string, logDst io.Writer) error {
	scanner := bufio.NewScanner(src)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // up to 10MB per line
	for scanner.Scan() {
		line := scanner.Bytes()
		ts := time.Now().Format("15:04:05.000")

		// Try to pretty-print the JSON-RPC message summary
		logMCPMessage(logDst, ts, direction, line)

		// Write through to destination (with newline)
		if _, err := dst.Write(line); err != nil {
			return err
		}
		if _, err := dst.Write([]byte("\n")); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// logMCPMessage parses a JSON-RPC message and logs a human-readable summary.
func logMCPMessage(w io.Writer, ts, direction string, data []byte) {
	var msg map[string]json.RawMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		fmt.Fprintf(w, "[%s] %s (%d bytes, non-JSON)\n", ts, direction, len(data))
		return
	}

	// Extract common fields
	method := unquote(msg["method"])
	id := string(msg["id"])

	if method != "" {
		// Request or notification
		var params map[string]json.RawMessage
		_ = json.Unmarshal(msg["params"], &params)

		if method == "tools/call" {
			// Show tool name and arguments summary
			toolName := unquote(params["name"])
			argSize := len(params["arguments"])
			fmt.Fprintf(w, "[%s] %s  %-14s tool=%s args=%dB id=%s\n",
				ts, direction, method, toolName, argSize, id)

			// Log full arguments for deep debugging
			if args, ok := params["arguments"]; ok {
				var pretty map[string]json.RawMessage
				if json.Unmarshal(args, &pretty) == nil {
					for k, v := range pretty {
						val := truncate(string(v), 120)
						fmt.Fprintf(w, "  ├─ %s: %s\n", k, val)
					}
				}
			}
		} else {
			fmt.Fprintf(w, "[%s] %s  %s id=%s\n", ts, direction, method, id)
		}
	} else if _, hasResult := msg["result"]; hasResult {
		// Response
		resultSize := len(msg["result"])
		fmt.Fprintf(w, "[%s] %s  response id=%s result=%dB\n",
			ts, direction, id, resultSize)

		// Show content summary for tool results
		var result map[string]json.RawMessage
		if json.Unmarshal(msg["result"], &result) == nil {
			if content, ok := result["content"]; ok {
				var items []map[string]json.RawMessage
				if json.Unmarshal(content, &items) == nil {
					for i, item := range items {
						text := truncate(unquote(item["text"]), 200)
						fmt.Fprintf(w, "  ├─ content[%d]: %s\n", i, text)
					}
				}
			}
		}
	} else if errField, hasErr := msg["error"]; hasErr {
		fmt.Fprintf(w, "[%s] %s  ERROR id=%s: %s\n",
			ts, direction, id, truncate(string(errField), 200))
	} else {
		fmt.Fprintf(w, "[%s] %s  (%d bytes)\n", ts, direction, len(data))
	}
}

func unquote(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return strings.Trim(string(raw), `"`)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
