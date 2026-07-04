# CS Native

<p align="center">
  <strong>README Language</strong><br />
  <a href="../../README.md">🇨🇳 中文</a> ·
  <a href="README.en.md">🇺🇸 English</a> ·
  <a href="README.ko.md">🇰🇷 한국어</a> ·
  <a href="README.ar.md">🇸🇦 العربية</a> ·
  <a href="README.de.md">🇩🇪 Deutsch</a> ·
  <a href="README.fr.md">🇫🇷 Français</a> ·
  <a href="README.la.md">🇻🇦 Latina</a>
</p>

<p align="center">
  Chinese is the default README. Click a flag to open another localized Markdown version.
</p>

<p align="center">
  <a href="https://github.com/eust-w/CSNative/actions/workflows/release.yml"><img src="https://github.com/eust-w/CSNative/actions/workflows/release.yml/badge.svg" alt="Release workflow" /></a>
  <a href="https://github.com/eust-w/CSNative/actions/workflows/pages.yml"><img src="https://github.com/eust-w/CSNative/actions/workflows/pages.yml/badge.svg" alt="Pages workflow" /></a>
  <a href="https://github.com/eust-w/CSNative/releases/latest"><img src="https://img.shields.io/github/v/release/eust-w/CSNative?label=release" alt="Latest release" /></a>
</p>

<p align="center">
  <img src="../../build/appicon.png" alt="CS Native icon" width="104" />
</p>

<p align="center">
  <strong>CS Native is a local provider/runtime control plane for Claude Science.</strong>
</p>

<p align="center">
  It runs Claude Science in an isolated sandbox, routes model requests to your configured third-party provider, and keeps the official Claude mode as a separate entry.
</p>

<p align="center">
  <img src="../assets/readme/csn-flow.gif" alt="CS Native flow animation" width="860" />
</p>

## What It Is

CS Native is not a single DeepSeek/Qwen switcher. It is a local control panel for:

- Managing provider profiles: base URL, API key, adapter, default model, and Science model mapping.
- Starting the local proxy, virtual OAuth, Claude Science sandbox, and fresh nonce public entry.
- Keeping third-party model mode inside an isolated sandbox.
- Keeping official Claude mode independent for the real Claude Science path.

## Basic Usage

1. Open **Models & Keys**.
2. Choose a built-in provider preset or create a custom provider.
3. Fill in `base URL`, `API key`, `adapter`, default model, and Science model mapping.
4. Click **Verify**.
5. Return to **Start**, select an enabled provider with a saved key, and click **Start**.
6. Switch to **Official Claude** when you want the real Claude Science path.

## i18n

CS Native defaults to Chinese and supports `zh-CN`, `en`, `ko`, `ar`, `de`, `fr`, and `la`.

- Desktop app: language selector in the sidebar.
- GitHub Pages: language selector in the header.
- README: static flag links, because GitHub Markdown does not run language-switching scripts.
- Arabic uses RTL layout in the app and website.

## Screens

### Start

![Start page](../assets/readme/hero-start.png)

### Models & Keys

![Models and keys](../assets/readme/provider-management.png)

### Network

![Network page](../assets/readme/network-public-entry.png)

### Logs

![Logs page](../assets/readme/runtime-logs.png)

## Download

Product page: [https://eust-w.github.io/CSNative/](https://eust-w.github.io/CSNative/)

Latest builds are available from [GitHub Releases](https://github.com/eust-w/CSNative/releases/latest).

| System / Arch | Asset | Content |
| --- | --- | --- |
| macOS Apple Silicon | `CSNative-v*-darwin-arm64.zip` | macOS `.app` bundle |
| macOS Intel | `CSNative-v*-darwin-amd64.zip` | macOS `.app` bundle |
| Windows x64 | `CSNative-v*-windows-amd64.zip` | Windows executable package |
| Linux x64 | `CSNative-v*-linux-amd64.tar.gz` | Linux executable package |
| All platforms | `checksums.txt` | SHA-256 checksums |

## Runtime Boundary

- Config directory: `~/.csnative`
- Sandbox HOME: `~/.csnative/sandbox/home`
- Local log: `~/.csnative/logs/csnative.log`
- Provider keys are stored in `~/.csnative/config.json` with `0600` permissions.
- CS Native does not write to the real Claude Science credential directory.

## Source

See the canonical Chinese README at [README.md](../../README.md), and the brand notes at [docs/brand.md](../brand.md).
