# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in Skillbox, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

Instead, email **security@devs-group.com** with:

1. A description of the vulnerability
2. Steps to reproduce
3. Potential impact
4. Suggested fix (if any)

We will acknowledge receipt within 48 hours and provide a timeline for a fix.

## Security Model

Skillbox enforces security at the runtime level. Every container execution receives the following controls, regardless of the caller:

- **Network isolation**: `NetworkMode: none` — no container can reach any network
- **Capability drop**: All Linux capabilities dropped
- **Read-only filesystem**: Container rootfs is read-only
- **PID limit**: Maximum 128 processes per container
- **No-new-privileges**: Prevents setuid/setgid escalation
- **Non-root user**: All containers run as user 65534 (nobody)
- **Docker socket proxy**: API communicates with Docker through a restricted proxy
- **Image allowlist**: Only pre-approved Docker images can be executed
- **Timeout enforcement**: Hard deadline via Go context cancellation

## Known Limitations

- **Shared kernel**: Docker containers share the host Linux kernel. For genuinely untrusted third-party code, consider enabling gVisor or Kata Containers as a Kubernetes RuntimeClass.
- **Docker daemon access**: The socket proxy sidecar has access to the Docker daemon. The proxy restricts which API calls are permitted, but a vulnerability in the proxy could theoretically allow broader access.

## Supported Versions

| Version | Supported |
|---|---|
| Latest release | Yes |
| Previous minor | Security fixes only |
| Older versions | No |
