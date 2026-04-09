# Troubleshooting

## Common Issues

### "no identity found"

**Cause:** DMGN requires an initialized identity before any operation.

**Fix:**
```bash
dmgn init
```

### "failed to load identity" / incorrect passphrase

**Cause:** Wrong passphrase entered, or identity file corrupted.

**Fix:**
- Re-enter the correct passphrase
- If passphrase is forgotten, restore from a key backup: `dmgn import -i backup.key`
- If no backup exists, re-initialize (data encrypted with old key will be inaccessible)

### MCP server not starting

**Cause:** Claude Desktop or Cline cannot find the `dmgn` binary.

**Fix:**
1. Ensure `dmgn` is on your PATH: `which dmgn` or `where dmgn`
2. Use absolute path in MCP config:
   ```json
   { "command": "/usr/local/bin/dmgn", "args": ["mcp-serve"] }
   ```
3. Check stderr output — stdout is reserved for JSON-RPC messages

### Query returns no results

**Possible causes:**
1. No memories stored — check with `get_status` tool or `dmgn status`
2. For semantic search, embeddings must be provided in both `add_memory` and `query_memory`
3. Text search requires word overlap with stored content

**Fix:** Verify `memory_count > 0` via `get_status`, then try broader queries.

### "failed to open badger db"

**Cause:** Database lock conflict — another DMGN process has the database open.

**Fix:**
1. Stop any running `dmgn start` or `dmgn mcp-serve` processes
2. If the process crashed, remove the lock file: `{data_dir}/storage/LOCK`

### Storage growing too large

**Fix:**
1. BadgerDB compacts automatically, but you can trigger GC by restarting the daemon
2. Check `max_recent_memories` config — lower values reduce stored data
3. Use `dmgn backup` before clearing data

### Peers not connecting

**Possible causes:**
1. Firewall blocking the listen port
2. No bootstrap peers configured
3. mDNS disabled or not working on your network

**Fix:**
1. Check `listen_addr` in config — use a specific port: `"/ip4/0.0.0.0/tcp/4001"`
2. Add bootstrap peers to config
3. Verify mDNS works: both nodes must be on the same local network
4. Check `dmgn peers` to see connection status

### Backup restore fails

**Cause:** Target directory already has data, or backup file is corrupted.

**Fix:**
1. Use `--force` flag: `dmgn restore backup.dmgn-backup --force`
2. Restore to a new directory: `dmgn restore backup.dmgn-backup --data-dir /new/path`
3. Verify backup file integrity — it should be a valid gzip tar archive

## Debug Logging

Enable debug logging for detailed output:

```bash
# Daemon mode
dmgn start --log-level debug

# MCP mode
dmgn mcp-serve --log-level debug
```

Log files are written to `{data_dir}/logs/dmgn.log` with automatic rotation.

## Getting Help

- Check logs: `{data_dir}/logs/dmgn.log`
- Run with debug: `--log-level debug`
- File issues: https://github.com/dmgn/dmgn/issues
