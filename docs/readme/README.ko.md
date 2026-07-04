# CS Native

<p align="center">
  <strong>README 언어</strong><br />
  <a href="../../README.md">🇨🇳 中文</a> ·
  <a href="README.en.md">🇺🇸 English</a> ·
  <a href="README.ko.md">🇰🇷 한국어</a> ·
  <a href="README.ar.md">🇸🇦 العربية</a> ·
  <a href="README.de.md">🇩🇪 Deutsch</a> ·
  <a href="README.fr.md">🇫🇷 Français</a> ·
  <a href="README.la.md">🇻🇦 Latina</a>
</p>

<p align="center">
  기본 README는 중국어입니다. 위 국기를 클릭하면 해당 언어의 Markdown 문서로 이동합니다.
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
  <strong>CS Native는 Claude Science용 로컬 provider/runtime control plane입니다.</strong>
</p>

<p align="center">
  Claude Science를 격리 샌드박스에서 실행하고, 모델 요청을 사용자가 설정한 타사 provider로 라우팅하며, 공식 Claude 모드는 별도入口로 유지합니다.
</p>

<p align="center">
  <img src="../assets/readme/csn-flow.gif" alt="CS Native 흐름 애니메이션" width="860" />
</p>

## 무엇인가

CS Native는 단일 DeepSeek/Qwen 스위처가 아닙니다. 다음을 관리하는 로컬 제어판입니다.

- Provider profile: base URL, API key, adapter, 기본 모델, Science 모델 매핑.
- 로컬 프록시, 가상 OAuth, Claude Science 샌드박스, fresh nonce 공개入口.
- 타사 모델 모드는 격리 샌드박스 안에서만 실행.
- 실제 Claude Science를 여는 공식 Claude 모드는 독립 유지.

## 기본 사용법

1. **모델과 자격 증명**을 엽니다.
2. 내장 provider preset을 선택하거나 사용자 지정 provider를 추가합니다.
3. `base URL`, `API key`, `adapter`, 기본 모델, Science 모델 매핑을 입력합니다.
4. **검증**을 클릭합니다.
5. **시작**으로 돌아가 저장된 key가 있는 provider를 선택하고 **시작**을 클릭합니다.
6. 실제 Claude Science가 필요하면 **공식 Claude** 모드로 전환합니다.

## i18n

CS Native의 기본 언어는 중국어이며 `zh-CN`, `en`, `ko`, `ar`, `de`, `fr`, `la`를 지원합니다.

- 데스크톱 앱: 사이드바의 언어 선택기.
- GitHub Pages: 헤더의 언어 선택기.
- README: GitHub Markdown이 스크립트 전환을 지원하지 않으므로 정적 국기 링크를 사용합니다.
- 아랍어는 앱과 웹사이트에서 RTL 레이아웃을 사용합니다.

## 화면

### 시작

![시작 페이지](../assets/readme/hero-start.png)

### 모델과 자격 증명

![모델과 자격 증명](../assets/readme/provider-management.png)

### 네트워크

![네트워크 페이지](../assets/readme/network-public-entry.png)

### 로그

![로그 페이지](../assets/readme/runtime-logs.png)

## 다운로드

제품 페이지: [https://eust-w.github.io/CSNative/](https://eust-w.github.io/CSNative/)

최신 빌드는 [GitHub Releases](https://github.com/eust-w/CSNative/releases/latest)에서 받을 수 있습니다.

| 시스템 / 아키텍처 | 자산 | 내용 |
| --- | --- | --- |
| macOS Apple Silicon | `CSNative-v*-darwin-arm64.zip` | macOS `.app` |
| macOS Intel | `CSNative-v*-darwin-amd64.zip` | macOS `.app` |
| Windows x64 | `CSNative-v*-windows-amd64.zip` | Windows 실행 파일 패키지 |
| Linux x64 | `CSNative-v*-linux-amd64.tar.gz` | Linux 실행 파일 패키지 |
| 전체 플랫폼 | `checksums.txt` | SHA-256 체크섬 |

## 실행 경계

- 설정 디렉터리: `~/.csnative`
- 샌드박스 HOME: `~/.csnative/sandbox/home`
- 로컬 로그: `~/.csnative/logs/csnative.log`
- Provider key는 `0600` 권한의 `~/.csnative/config.json`에 저장됩니다.
- CS Native는 실제 Claude Science 자격 증명 디렉터리에 쓰지 않습니다.

## 소스

기준 문서는 중국어 [README.md](../../README.md)이며, 브랜드 설명은 [docs/brand.md](../brand.md)에 있습니다.
