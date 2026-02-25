# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest  | ✅        |
| < latest | ❌       |

Only the latest release receives security updates.

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly:

1. **Do not** open a public GitHub issue
2. Email **security@stuffbucket.com** with a description of the vulnerability
3. Include steps to reproduce if possible
4. Allow up to 72 hours for an initial response

We will coordinate disclosure and credit reporters in the release notes.

## Supply Chain

- Go binaries are built in CI from tagged commits via GitHub Actions
- Release binaries include a `checksums.txt` (SHA-256) for integrity verification
- The npm postinstall script verifies checksums before executing downloaded binaries
- npm packages are published with [provenance](https://docs.npmjs.com/generating-provenance-statements) for supply chain transparency
- Dependencies are scanned with `govulncheck` and `npm audit` before each release
