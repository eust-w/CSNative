# CS Native

<p align="center">
  <strong>Langue du README</strong><br />
  <a href="../../README.md">🇨🇳 中文</a> ·
  <a href="README.en.md">🇺🇸 English</a> ·
  <a href="README.ko.md">🇰🇷 한국어</a> ·
  <a href="README.ar.md">🇸🇦 العربية</a> ·
  <a href="README.de.md">🇩🇪 Deutsch</a> ·
  <a href="README.fr.md">🇫🇷 Français</a> ·
  <a href="README.la.md">🇻🇦 Latina</a>
</p>

<p align="center">
  Le README par defaut est en chinois. Cliquez sur un drapeau pour ouvrir la version Markdown correspondante.
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
  <strong>CS Native est un provider/runtime control plane local pour Claude Science.</strong>
</p>

<p align="center">
  Il execute Claude Science dans un bac a sable isole, route les requetes de modele vers votre provider tiers configure, et garde le mode Claude officiel comme entree separee.
</p>

<p align="center">
  <img src="../assets/readme/csn-flow.gif" alt="Animation du flux CS Native" width="860" />
</p>

## Presentation

CS Native n'est pas un simple commutateur DeepSeek/Qwen. C'est un panneau de controle local pour :

- Gerer les provider profiles : base URL, API key, adapter, modele par defaut et correspondance des modeles Science.
- Demarrer le proxy local, l'OAuth virtuel, le bac a sable Claude Science et l'entree publique fresh nonce.
- Garder le mode modele tiers dans un bac a sable isole.
- Conserver un mode Claude officiel independant pour le vrai chemin Claude Science.

## Utilisation De Base

1. Ouvrez **Modeles et cles**.
2. Choisissez un provider preset integre ou creez un provider personnalise.
3. Renseignez `base URL`, `API key`, `adapter`, modele par defaut et correspondance des modeles Science.
4. Cliquez sur **Verifier**.
5. Revenez a **Demarrer**, choisissez un provider active avec une key enregistree, puis cliquez sur **Demarrer**.
6. Passez en mode **Claude officiel** pour utiliser le vrai Claude Science.

## i18n

CS Native utilise le chinois par defaut et prend en charge `zh-CN`, `en`, `ko`, `ar`, `de`, `fr` et `la`.

- Application desktop : selecteur de langue dans la barre laterale.
- GitHub Pages : selecteur de langue dans l'en-tete.
- README : liens de drapeaux statiques, car GitHub Markdown n'execute pas de scripts de changement de langue.
- L'arabe utilise une mise en page RTL dans l'application et le site.

## Interfaces

### Demarrage

![Page de demarrage](../assets/readme/hero-start.png)

### Modeles et cles

![Modeles et cles](../assets/readme/provider-management.png)

### Reseau

![Page reseau](../assets/readme/network-public-entry.png)

### Journaux

![Journaux](../assets/readme/runtime-logs.png)

## Telechargement

Page produit : [https://eust-w.github.io/CSNative/](https://eust-w.github.io/CSNative/)

Les dernieres versions sont disponibles dans [GitHub Releases](https://github.com/eust-w/CSNative/releases/latest).

| Systeme / Architecture | Asset | Contenu |
| --- | --- | --- |
| macOS Apple Silicon | `CSNative-v*-darwin-arm64.zip` | Bundle macOS `.app` |
| macOS Intel | `CSNative-v*-darwin-amd64.zip` | Bundle macOS `.app` |
| Windows x64 | `CSNative-v*-windows-amd64.zip` | Package executable Windows |
| Linux x64 | `CSNative-v*-linux-amd64.tar.gz` | Package executable Linux |
| Toutes plateformes | `checksums.txt` | Sommes SHA-256 |

## Limites D'execution

- Dossier de configuration : `~/.csnative`
- Sandbox HOME : `~/.csnative/sandbox/home`
- Journal local : `~/.csnative/logs/csnative.log`
- Les provider keys sont stockees dans `~/.csnative/config.json` avec les permissions `0600`.
- CS Native n'ecrit pas dans le vrai dossier d'identifiants Claude Science.

## Source

Le README chinois canonique est [README.md](../../README.md), et les notes de marque sont dans [docs/brand.md](../brand.md).
