# Security notes

## What stays local

The browser talks only to the Go process bound to `127.0.0.1`. The Go process talks directly to Google's OAuth and Gmail APIs. It does not call Vercel, an analytics service, an LLM, or any application-owned server.

Raw message content and OAuth access tokens are held only in memory. The refresh token and the encryption key are saved as separate entries in macOS Keychain. The local job list contains company, role, inferred status, subject, Gmail message ID, and date; it is encrypted with AES-256-GCM before being saved.

## Deliberate limits

- Gmail access is read-only. The app cannot send, delete, label, or alter mail.
- The parser requests metadata and Gmail's short snippet, not full MIME message bodies.
- The server listens on loopback, not the LAN.
- OAuth callbacks use a random, expiring state value.
- State-changing API calls reject foreign browser origins.

## Threat boundary

This design keeps data away from hosting providers and other accounts. It cannot protect the mailbox from malware, a compromised browser, or another process already running with the same logged-in macOS user's permissions. Keep macOS and the browser updated, use FileVault, and do not commit `.env`.

If this becomes a multi-user public product, stop and redesign the storage, identity, Google verification, privacy policy, deletion, audit, and incident-response model before deployment.
