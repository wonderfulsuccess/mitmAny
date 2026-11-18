# Roadmap

This is a direction, not a delivery-date commitment. Discuss substantial work before implementing it.

## Before the first public release

- Complete the [platform acceptance checklist](release-checklist.md) with signed/unsigned package status recorded.
- Confirm asset provenance and run history/working-tree secret scans.
- Publish reproducible downloads, screenshots, privacy and recovery documentation.

## Reliability and usability

- Dedicated macOS/Windows recovery regression coverage for VPN/PAC and sleep/wake scenarios.
- Better storage diagnostics and configurable bounded retention.
- Improve localization and keyboard/accessibility coverage.
- Investigate the deprecated macOS default-keychain API without weakening trust handling.

## Protocol analysis

- WebSocket message inspection with bounded storage.
- Better evidence export and explicit evidence retention.
- Optional structured body queries and gRPC/Protobuf support.
- Clearer source attribution when desktop and Android share the store.

## Requires separate design

Request replay, rewriting, TUN interception and any remote access would change the safety/architecture boundaries. They are not promised by the current release.
