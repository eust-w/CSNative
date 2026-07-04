# CS Native

<p align="center">
  <strong>README-Sprache</strong><br />
  <a href="../../README.md">🇨🇳 中文</a> ·
  <a href="README.en.md">🇺🇸 English</a> ·
  <a href="README.ko.md">🇰🇷 한국어</a> ·
  <a href="README.ar.md">🇸🇦 العربية</a> ·
  <a href="README.de.md">🇩🇪 Deutsch</a> ·
  <a href="README.fr.md">🇫🇷 Français</a> ·
  <a href="README.la.md">🇻🇦 Latina</a>
</p>

<p align="center">
  Chinesisch ist die Standard-README. Klicke auf eine Flagge, um die jeweilige Markdown-Version zu öffnen.
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
  <strong>CS Native ist ein lokales provider/runtime control plane fuer Claude Science.</strong>
</p>

<p align="center">
  Es startet Claude Science in einer isolierten Sandbox, leitet Modellanfragen an deinen konfigurierten Drittanbieter-provider weiter und behaelt den offiziellen Claude-Modus als getrennten Einstieg.
</p>

<p align="center">
  <img src="../assets/readme/csn-flow.gif" alt="CS Native Ablaufanimation" width="860" />
</p>

## Was Es Ist

CS Native ist kein einzelner DeepSeek/Qwen-Umschalter. Es ist ein lokales Control Panel fuer:

- Provider profiles: base URL, API key, adapter, Standardmodell und Science-Modellzuordnung.
- Lokalen Proxy, virtuelles OAuth, Claude-Science-Sandbox und fresh-nonce public entry.
- Drittanbieter-Modus nur innerhalb einer isolierten Sandbox.
- Einen separaten offiziellen Claude-Modus fuer den echten Claude-Science-Pfad.

## Grundlegende Nutzung

1. Oeffne **Modelle & Zugangsdaten**.
2. Waehle ein eingebautes provider preset oder erstelle einen eigenen provider.
3. Trage `base URL`, `API key`, `adapter`, Standardmodell und Science-Modellzuordnung ein.
4. Klicke **Verifizieren**.
5. Gehe zurueck zu **Start**, waehle einen aktivierten provider mit gespeichertem key und klicke **Start**.
6. Wechsle zu **Offizieller Claude**, wenn du den echten Claude-Science-Pfad nutzen willst.

## i18n

CS Native nutzt Chinesisch als Standardsprache und unterstuetzt `zh-CN`, `en`, `ko`, `ar`, `de`, `fr` und `la`.

- Desktop-App: Sprachauswahl in der Seitenleiste.
- GitHub Pages: Sprachauswahl im Header.
- README: statische Flaggenlinks, weil GitHub Markdown keine Sprachwechsel-Skripte ausfuehrt.
- Arabisch nutzt RTL-Layout in App und Website.

## Oberflaechen

### Start

![Startseite](../assets/readme/hero-start.png)

### Modelle & Zugangsdaten

![Modelle und Zugangsdaten](../assets/readme/provider-management.png)

### Netzwerk

![Netzwerkseite](../assets/readme/network-public-entry.png)

### Logs

![Logs](../assets/readme/runtime-logs.png)

## Download

Produktseite: [https://eust-w.github.io/CSNative/](https://eust-w.github.io/CSNative/)

Aktuelle Builds liegen in den [GitHub Releases](https://github.com/eust-w/CSNative/releases/latest).

| System / Architektur | Asset | Inhalt |
| --- | --- | --- |
| macOS Apple Silicon | `CSNative-v*-darwin-arm64.zip` | macOS `.app` |
| macOS Intel | `CSNative-v*-darwin-amd64.zip` | macOS `.app` |
| Windows x64 | `CSNative-v*-windows-amd64.zip` | Windows-Paket |
| Linux x64 | `CSNative-v*-linux-amd64.tar.gz` | Linux-Paket |
| Alle Plattformen | `checksums.txt` | SHA-256-Pruefsummen |

## Laufzeitgrenzen

- Konfigurationsordner: `~/.csnative`
- Sandbox HOME: `~/.csnative/sandbox/home`
- Lokales Log: `~/.csnative/logs/csnative.log`
- Provider keys werden in `~/.csnative/config.json` mit `0600` Rechten gespeichert.
- CS Native schreibt nicht in den echten Claude-Science-Credential-Ordner.

## Quelle

Die kanonische chinesische README liegt unter [README.md](../../README.md), Markennotizen unter [docs/brand.md](../brand.md).
