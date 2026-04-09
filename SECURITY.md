# Security Policy

## Supported Versions

The following versions of DMGN are currently supported with security updates:

| Version | Supported          |
|---------|-------------------|
| 0.1.x   | :white_check_mark:|
| < 0.1.0 | :x:              |

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly.

### Do NOT

- Open a public GitHub issue
- Discuss the vulnerability publicly
- Include sensitive data in bug reports

### DO

1. **Email**: Send details to the maintainers privately
2. **Include**:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Any suggested fixes (optional)
3. **Wait**: for acknowledgment before public disclosure

### Response Timeline

| Timeline | Action |
|----------|--------|
| 24 hours | Acknowledgment of report |
| 7 days   | Initial assessment and status |
| 30 days  | Fix or mitigation plan |
| 90 days  | Public disclosure (if applicable) |

## Security Features

### Encryption

- **Key derivation**: Argon2id (memory-hard)
- **Identity encryption**: XChaCha20-Poly1305
- **Memory encryption**: AES-GCM-256 with per-memory keys
- **Key hierarchy**: Master key → per-memory keys

### Known Security Considerations

1. **Passphrase strength**: Minimum 8 characters recommended
2. **Data directory permissions**: Ensure proper file permissions
3. **Network exposure**: Restrict API access in production
4. **Backup security**: Encrypted backups are safe to store

## Security Updates

Subscribe to the repository to receive security advisories:

```bash
# Watch releases only
git fetch origin && git log --oneline --all | head -20
```

## Attribution

Thank you for helping keep DMGN secure!