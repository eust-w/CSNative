package desktop

import (
	"os"
	"strings"
	"testing"
)

func TestStartProviderSelectUsesConfiguredProviders(t *testing.T) {
	js, err := os.ReadFile("frontend/dist/main.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(js)
	for _, want := range []string{
		`api.ListProviders()`,
		`function renderStartProviderSelect`,
		`let startProviderId = ""`,
		`function launchProviders()`,
		`return providers.filter((p) => p.enabled && p.has_key);`,
		`startProvider.addEventListener("change"`,
		`api.StartWithProvider(providerID)`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("startup provider flow missing %q", want)
		}
	}
}

func TestStartProviderSelectionIsSeparateFromProviderEditor(t *testing.T) {
	js, err := os.ReadFile("frontend/dist/main.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(js)
	for _, want := range []string{
		`function startProvider()`,
		`const launchNote = p.id === startProviderId ? ` + "` · ${t(\"provider.launch\")}`" + ` : (p.enabled && !p.has_key ? ` + "` · ${t(\"provider.notLaunchable\")}`" + ` : "");`,
		`renderProviderEditor(p);`,
		`const providerID = els.startProvider.value || startProviderId;`,
		`throw new Error(t("msg.defaultNeedsKey"));`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("startup/editor provider separation missing %q", want)
		}
	}
	if strings.Contains(src, `const list = enabledProviders();`) {
		t.Fatal("startup provider dropdown must not include enabled providers that still lack keys")
	}
}

func TestToolbarButtonsHaveVisibleFeedbackAndErrorHandling(t *testing.T) {
	js, err := os.ReadFile("frontend/dist/main.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(js)
	if !strings.Contains(src, `async function runAction`) {
		t.Fatal("frontend should route button actions through runAction for visible status and errors")
	}
	for _, want := range []string{
		`openBrowserBtn.addEventListener("click", () => runAction(t("action.openBrowser")`,
		`doctorBtn.addEventListener("click", () => runAction(t("action.doctor")`,
		`openLogDirBtn.addEventListener("click", () => runAction(t("action.openLogDir")`,
		`reportBtn.addEventListener("click", () => runAction(t("action.report")`,
		`updateBtn.addEventListener("click", () => runAction(t("action.update")`,
		`quitBtn.addEventListener("click", () => runAction(t("action.quit")`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("missing wrapped handler containing %q", want)
		}
	}
}

func TestModeButtonsUseVisibleErrorHandling(t *testing.T) {
	js, err := os.ReadFile("frontend/dist/main.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(js)
	for _, want := range []string{
		`return runAction(t("msg.openOfficial")`,
		`runAction(t("action.switchMode")`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("mode flow should use visible error handling containing %q", want)
		}
	}
}

func TestPrimaryActionsFormatCaughtErrors(t *testing.T) {
	js, err := os.ReadFile("frontend/dist/main.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(js)
	for _, want := range []string{
		`t("msg.providerSaveFailed", { error: errorText(e) })`,
		`t("msg.startFailed", { error: errorText(e) })`,
		`t("msg.stopFailed", { error: errorText(e) })`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("primary action catch should format errors with %q", want)
		}
	}
}

func TestBrowserPreviewFallsBackToMockAPIWithoutWailsRuntime(t *testing.T) {
	js, err := os.ReadFile("frontend/dist/main.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(js)
	if !strings.Contains(src, `window.go?.app?.App`) {
		t.Fatal("browser preview should detect missing Wails runtime and use mock API")
	}
}

func TestPanelElementHasExpectedID(t *testing.T) {
	html, err := os.ReadFile("frontend/dist/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), `<main id="panel" class="panel">`) {
		t.Fatal("main panel must expose id=panel for frontend mode switching")
	}
}

func TestDesktopWindowStartsInControlPanelLayout(t *testing.T) {
	goSrc, err := os.ReadFile("run.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(goSrc)
	for _, want := range []string{
		`Width:     960,`,
		`Height:    832,`,
		`MinWidth:  760,`,
		`MinHeight: 620,`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("desktop launch window should match adapted control panel layout, missing %q", want)
		}
	}
}

func TestNativeSettingsLayoutIncludesSidebarAndPublicStatus(t *testing.T) {
	html, err := os.ReadFile("frontend/dist/index.html")
	if err != nil {
		t.Fatal(err)
	}
	src := string(html)
	for _, want := range []string{
		`<aside class="sidebar"`,
		`data-tab="startPane"`,
		`data-tab="providerPane"`,
		`data-tab="networkPane"`,
		`data-tab="logPane"`,
		`data-tab="aboutPane"`,
		`id="startProvider"`,
		`id="ltPublic"`,
		`公网入口`,
		`id="publicBaseURL"`,
		`id="publicURLPreview"`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("native settings layout missing %q", want)
		}
	}
}

func TestNativeSettingsTabsHaveDistinctPanes(t *testing.T) {
	html, err := os.ReadFile("frontend/dist/index.html")
	if err != nil {
		t.Fatal(err)
	}
	src := string(html)
	for _, want := range []string{
		`id="startPane"`,
		`id="providerPane"`,
		`id="networkPane"`,
		`id="logPane"`,
		`id="aboutPane"`,
		`id="verifyProviderBtn"`,
		`id="providerBaseURLInput"`,
		`id="providerKeyInput"`,
		`id="saveNetworkBtn"`,
		`id="copyPublicURLBtn"`,
		`id="logOutput"`,
		`id="refreshLogBtn"`,
		`id="diagnosticOutput"`,
		`模型与凭证`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("tabbed layout missing %q", want)
		}
	}
}

func TestLogPageReadsBackendLogsSeparatelyFromAbout(t *testing.T) {
	html, err := os.ReadFile("frontend/dist/index.html")
	if err != nil {
		t.Fatal(err)
	}
	js, err := os.ReadFile("frontend/dist/main.js")
	if err != nil {
		t.Fatal(err)
	}
	htmlSrc := string(html)
	jsSrc := string(js)
	for _, want := range []string{
		`id="logPane"`,
		`id="logOutput"`,
		`id="aboutPane"`,
		`id="doctorBtn"`,
		`api.ReadLogs()`,
		`api.ClearLogs()`,
		`if (paneId === "logPane") refreshLogs();`,
	} {
		if !strings.Contains(htmlSrc+jsSrc, want) {
			t.Fatalf("log/about separation missing %q", want)
		}
	}
	logPaneStart := strings.Index(htmlSrc, `id="logPane"`)
	aboutPaneStart := strings.Index(htmlSrc, `id="aboutPane"`)
	if logPaneStart < 0 || aboutPaneStart < 0 || logPaneStart > aboutPaneStart {
		t.Fatalf("expected log pane before about pane")
	}
	logPaneMarkup := htmlSrc[logPaneStart:aboutPaneStart]
	if strings.Contains(logPaneMarkup, `doctorBtn`) || strings.Contains(logPaneMarkup, `reportBtn`) {
		t.Fatal("log pane should not contain about/diagnostic actions")
	}
}

func TestNavigationUsesRealTabSwitching(t *testing.T) {
	js, err := os.ReadFile("frontend/dist/main.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(js)
	for _, want := range []string{
		`function showTab`,
		`item.hidden = !active`,
		`button.dataset.tab`,
		`aria-selected`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("tab navigation missing %q", want)
		}
	}
	if strings.Contains(src, `scrollIntoView`) {
		t.Fatal("sidebar navigation must switch tabs, not scroll to anchors")
	}
}

func TestFrontendRefreshesPublicStatusLight(t *testing.T) {
	js, err := os.ReadFile("frontend/dist/main.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(js)
	for _, want := range []string{
		`ltPublic`,
		`setLight(els.ltPublic, s.public)`,
		`public_base_url: els.publicBaseURL.value.trim()`,
		`api.OpenPublicURL()`,
		`reflectSummary`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("frontend public status handling missing %q", want)
		}
	}
}

func TestProviderEditorTracksBackendProfiles(t *testing.T) {
	js, err := os.ReadFile("frontend/dist/main.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(js)
	for _, want := range []string{
		`function collectProvider`,
		`model_map`,
		`api.SaveProvider(collectProvider())`,
		`api.VerifyProvider(saved.id)`,
		`api.SetActiveProvider(saved.id)`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("provider editor missing %q", want)
		}
	}
}
