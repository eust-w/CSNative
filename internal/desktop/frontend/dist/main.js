let api = null;
let runtimeAPI = null;
let busy = false;
let mode = "proxy";
let activePane = "startPane";
let providers = [];
let selectedProviderId = "";
let startProviderId = "";

async function loadAPI() {
  try {
    const mod = await import("./wailsjs/go/app/App.js");
    return window.go?.app?.App ? mod : mockAPI;
  } catch {
    return mockAPI;
  }
}

async function loadRuntime() {
  if (runtimeAPI) return runtimeAPI;
  try {
    runtimeAPI = await import("./wailsjs/runtime/runtime.js");
  } catch {
    runtimeAPI = { Quit: () => window.runtime?.Quit?.() };
  }
  return runtimeAPI;
}

const mockAPI = {
  GetConfig: async () => ({ provider: "qwen", proxy_port: 18991, sandbox_port: 8990, public_port: 8992, public_base_url: "", mode: "proxy", keys: {} }),
  ListProviders: async () => mockProviders(),
  SaveProvider: async (p) => ({ ...p, api_key: "", has_key: !!p.api_key, key_masked: p.api_key ? "••••" + p.api_key.slice(-4) : "", active: false }),
  DeleteProvider: async () => true,
  SetActiveProvider: async () => true,
  VerifyProvider: async () => ({ ok: true, hint: "预览模式" }),
  StartWithProvider: async () => ({ url: "http://127.0.0.1:8990", msg: "预览模式" }),
  SetConfig: async () => true,
  SetMode: async () => true,
  OneClickLogin: async () => ({ url: "http://127.0.0.1:8990", msg: "预览模式" }),
  StopAll: async () => null,
  Status: async () => ({ proxy: "amber", sandbox: "amber", upstream: "amber", public: "amber" }),
  OpenURL: async () => null,
  OpenPublicURL: async () => null,
  RunDoctor: async () => "预览模式",
  OpenOfficial: async () => null,
  OpenLogs: async () => null,
  ReadLogs: async () => `${new Date().toISOString()} [INFO] preview log initialized\n${new Date().toISOString()} [INFO] provider saved id=qwen adapter=openai_chat_completions enabled=true`,
  ClearLogs: async () => true,
  OpenReleasePage: async () => null,
  ReportBug: async () => null,
  AppVersion: async () => "0.1.0-preview",
};

function mockProviders() {
  const deepseek = providerTemplate("deepseek", "DeepSeek", "anthropic_messages", "https://api.deepseek.com/anthropic/v1/messages", "deepseek-v4-flash", true, false);
  const qwen = providerTemplate("qwen", "阿里云 DashScope / Qwen", "openai_chat_completions", "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions", "qwen-plus-latest", true, true);
  qwen.has_key = true;
  qwen.key_masked = "••••test";
  qwen.verified = true;
  return [deepseek, qwen];
}

const $ = (id) => document.getElementById(id);
const els = {};
const scienceModels = {
  opus: "claude-opus-4-8",
  sonnet: "claude-sonnet-4-6",
  haiku: "claude-haiku-4-5",
};

function setMsg(text, kind = "") {
  els.msg.textContent = text;
  els.msg.className = "msg" + (kind ? " " + kind : "");
}
function setDiagnostic(text) {
  if (els.diagnosticOutput) els.diagnosticOutput.textContent = text || "诊断输出为空。";
}
function setLogOutput(text) {
  if (els.logOutput) els.logOutput.textContent = text || "暂无日志。";
}
function errorText(err) {
  if (!err) return "未知错误";
  if (typeof err === "string") return err;
  return err.message || String(err);
}
function setBusy(on) {
  busy = on;
  [
    els.oneClickBtn, els.stopBtn, els.saveNetworkBtn, els.saveProviderBtn, els.verifyProviderBtn,
    els.setDefaultProviderBtn, els.deleteProviderBtn, els.addProviderBtn, els.copyProviderBtn,
    els.refreshLogBtn, els.openLogDirBtn, els.clearLogBtn,
  ].filter(Boolean).forEach((b) => (b.disabled = on));
}
function setLight(el, state) {
  if (el) el.className = "lt " + ({ green: "g", amber: "a", red: "r" }[state] || "a");
}
function providerById(id) {
  return providers.find((p) => p.id === id) || null;
}
function currentProvider() {
  return providers.find((p) => p.id === selectedProviderId) || providers.find((p) => p.active) || providers[0] || null;
}
function startProvider() {
  return providerById(startProviderId) || providers.find((p) => p.active && p.enabled && p.has_key) || launchProviders()[0] || null;
}
function launchProviders() {
  return providers.filter((p) => p.enabled && p.has_key);
}
function providerStatus(provider) {
  if (!provider) return "missing";
  if (!provider.has_key) return "缺 key";
  if (provider.verified) return "已验证";
  return "未验证";
}
function statusClass(provider) {
  if (!provider || !provider.has_key) return "err";
  return provider.verified ? "ok" : "";
}
function adapterLabel(adapter) {
  return adapter === "anthropic_messages" ? "Anthropic Messages" : "OpenAI Chat Completions";
}

function applyMode(next) {
  mode = next === "official" ? "official" : "proxy";
  els.panel.classList.toggle("mode-official", mode === "official");
  els.modeSeg.querySelectorAll(".seg-btn").forEach((b) => b.classList.toggle("active", b.dataset.mode === mode));
  els.oneClickBtn.textContent = mode === "official" ? "打开官方 Claude Science" : "一键开始";
  if (mode === "official" && $(activePane)?.classList.contains("tp-only")) showTab("startPane");
}

function showTab(paneId) {
  const pane = $(paneId);
  if (!pane) return;
  if (mode === "official" && pane.classList.contains("tp-only")) {
    setMsg("官方模式下仅保留启动页。请切回第三方模型后再配置模型、凭证或网络。", "err");
    paneId = "startPane";
  }
  activePane = paneId;
  document.querySelectorAll(".tab-pane").forEach((item) => {
    const active = item.id === paneId;
    item.classList.toggle("active", active);
    item.hidden = !active;
  });
  document.querySelectorAll(".nav-item").forEach((button) => {
    const active = button.dataset.tab === paneId;
    button.classList.toggle("active", active);
    button.setAttribute("aria-selected", active ? "true" : "false");
  });
  const button = document.querySelector(`.nav-item[data-tab="${paneId}"]`);
  if (button) {
    els.pageTitle.textContent = button.dataset.title || button.textContent.trim();
    els.pageDesc.textContent = button.dataset.desc || "";
  }
  if (paneId === "logPane") refreshLogs();
}

function settings() {
  return {
    provider: els.startProvider.value || startProviderId || selectedProviderId || "deepseek",
    proxy_port: parseInt(els.proxyPort.value, 10) || 18991,
    sandbox_port: parseInt(els.sandboxPort.value, 10) || 8990,
    public_port: parseInt(els.publicPort.value, 10) || 8992,
    public_base_url: els.publicBaseURL.value.trim(),
  };
}
function reflectSummary() {
  if (els.proxyStatusValue) els.proxyStatusValue.textContent = String(parseInt(els.proxyPort.value, 10) || 18991);
  if (els.sandboxStatusValue) els.sandboxStatusValue.textContent = String(parseInt(els.sandboxPort.value, 10) || 8990);
  if (els.publicStatusValue) els.publicStatusValue.textContent = String(parseInt(els.publicPort.value, 10) || 8992);
  if (els.upstreamStatusValue) {
    const p = startProvider() || currentProvider();
    els.upstreamStatusValue.textContent = p ? p.display_name : "未配置";
  }
  const publicPort = parseInt(els.publicPort.value, 10) || 8992;
  const sandboxPort = parseInt(els.sandboxPort.value, 10) || 8990;
  const publicURL = els.publicBaseURL.value.trim().replace(/\/+$/, "") || `http://127.0.0.1:${publicPort}`;
  if (els.publicURLPreview) els.publicURLPreview.textContent = publicURL;
  if (els.publicRootTarget) els.publicRootTarget.textContent = `127.0.0.1:${publicPort}`;
  if (els.publicSandboxTarget) els.publicSandboxTarget.textContent = `127.0.0.1:${sandboxPort}`;
  if (els.publicRouteSummary) els.publicRouteSummary.textContent = `${publicURL}/ 生成 fresh nonce；其它路径转发到沙箱`;
}
async function persistSettings() {
  await api.SetConfig(settings());
  reflectSummary();
}
async function runAction(label, fn, successText) {
  setMsg(label + "…");
  try {
    const result = await fn();
    const text = typeof successText === "function" ? successText(result) : successText;
    if (text) setMsg(text, "ok");
    return result;
  } catch (err) {
    setMsg(label + "失败：" + errorText(err), "err");
    return null;
  }
}

async function loadConfig() {
  const cfg = await api.GetConfig();
  els.proxyPort.value = cfg.proxy_port || 18991;
  els.sandboxPort.value = cfg.sandbox_port || 8990;
  els.publicPort.value = cfg.public_port || 8992;
  els.publicBaseURL.value = cfg.public_base_url || "";
  applyMode(cfg.mode);
  await loadProviders(cfg.provider);
  reflectSummary();
}

async function loadProviders(preferredId = "") {
  providers = await api.ListProviders();
  const activeId = providers.find((p) => p.active)?.id || "";
  if (preferredId && providerById(preferredId)) {
    selectedProviderId = preferredId;
  } else if (!providerById(selectedProviderId)) {
    selectedProviderId = activeId || providers[0]?.id || "";
  }
  const launchable = launchProviders();
  if (preferredId && launchable.find((p) => p.id === preferredId)) {
    startProviderId = preferredId;
  } else if (startProviderId && launchable.find((p) => p.id === startProviderId)) {
    // Keep the user's current launch choice.
  } else if (activeId && launchable.find((p) => p.id === activeId)) {
    startProviderId = activeId;
  } else {
    startProviderId = launchable[0]?.id || "";
  }
  renderStartProviderSelect();
  renderProviderList();
  renderProviderEditor(currentProvider());
  reflectSummary();
}

function renderStartProviderSelect() {
  const list = launchProviders();
  els.startProvider.innerHTML = "";
  if (list.length === 0) {
    const opt = document.createElement("option");
    opt.value = "";
    opt.textContent = "没有可启动 provider";
    els.startProvider.appendChild(opt);
    els.startProvider.disabled = true;
    startProviderId = "";
    els.startProviderSummary.textContent = "请先到“模型与凭证”启用 provider，并保存 API key。";
    els.oneClickBtn.disabled = true;
    return;
  }
  els.startProvider.disabled = false;
  els.oneClickBtn.disabled = busy;
  for (const p of list) {
    const opt = document.createElement("option");
    opt.value = p.id;
    opt.textContent = `${p.display_name} · ${providerStatus(p)} · ${p.default_model || "未设模型"}`;
    els.startProvider.appendChild(opt);
  }
  if (!list.find((p) => p.id === startProviderId)) startProviderId = list[0].id;
  els.startProvider.value = startProviderId;
  updateStartProviderSummary();
}

function updateStartProviderSummary() {
  const p = startProvider();
  if (!p) {
    els.startProviderSummary.textContent = "请先配置一个可启动 provider。";
    return;
  }
  els.startProviderSummary.innerHTML = `
    <span class="state-badge ${statusClass(p)}">${providerStatus(p)}</span>
    <strong>${escapeHTML(p.display_name)}</strong>
    <em>${escapeHTML(adapterLabel(p.adapter))}</em>
    <code>${escapeHTML(p.default_model || "")}</code>
    <small>${escapeHTML(p.base_url || "")}</small>
  `;
}

function renderProviderList() {
  els.providerList.innerHTML = "";
  for (const p of providers) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "provider-list-item" + (p.id === selectedProviderId ? " active" : "");
    const launchNote = p.id === startProviderId ? " · 启动" : (p.enabled && !p.has_key ? " · 不可启动" : "");
    button.innerHTML = `<span class="mini-dot ${p.has_key ? (p.verified ? "g" : "a") : "r"}"></span><strong>${escapeHTML(p.display_name)}</strong><em>${escapeHTML(providerStatus(p) + launchNote)}</em>`;
    button.addEventListener("click", () => {
      selectedProviderId = p.id;
      renderProviderList();
      renderProviderEditor(p);
    });
    els.providerList.appendChild(button);
  }
}

function renderProviderEditor(provider) {
  if (!provider) {
    els.providerEditorTitle.textContent = "Provider";
    return;
  }
  els.providerEditorTitle.textContent = provider.display_name || provider.id;
  els.providerStateBadge.textContent = providerStatus(provider);
  els.providerStateBadge.className = "state-badge " + statusClass(provider);
  els.providerIdInput.value = provider.id || "";
  els.providerIdInput.disabled = !!provider.builtin;
  els.providerNameInput.value = provider.display_name || "";
  els.providerAdapterInput.value = provider.adapter || "openai_chat_completions";
  els.providerBaseURLInput.value = provider.base_url || "";
  els.providerDefaultModelInput.value = provider.default_model || "";
  els.providerKeyInput.value = "";
  els.providerKeyInput.placeholder = provider.has_key ? "已保存：" + provider.key_masked + "；留空保留" : "粘贴 API key";
  els.mapOpusInput.value = provider.model_map?.[scienceModels.opus] || provider.default_model || "";
  els.mapSonnetInput.value = provider.model_map?.[scienceModels.sonnet] || provider.model_map?.["claude-sonnet-5"] || provider.default_model || "";
  els.mapHaikuInput.value = provider.model_map?.[scienceModels.haiku] || provider.default_model || "";
  els.providerEnabledInput.checked = !!provider.enabled;
}

function collectProvider() {
  const model = els.providerDefaultModelInput.value.trim();
  const mapOpus = els.mapOpusInput.value.trim() || model;
  const mapSonnet = els.mapSonnetInput.value.trim() || model;
  const mapHaiku = els.mapHaikuInput.value.trim() || model;
  const models = [...new Set([model, mapOpus, mapSonnet, mapHaiku].filter(Boolean))].map((id) => ({ id, display_name: id }));
  return {
    id: els.providerIdInput.value.trim(),
    display_name: els.providerNameInput.value.trim(),
    adapter: els.providerAdapterInput.value,
    base_url: els.providerBaseURLInput.value.trim(),
    api_key: els.providerKeyInput.value.trim(),
    default_model: model,
    models,
    model_map: {
      "claude-opus-4-8": mapOpus,
      "claude-sonnet-5": mapSonnet,
      "claude-sonnet-4-6": mapSonnet,
      "claude-haiku-4-5": mapHaiku,
    },
	    max_tokens_cap: Object.fromEntries(models.map((m) => [m.id, 8192])),
	    enabled: els.providerEnabledInput.checked,
	    disabled: !els.providerEnabledInput.checked,
	    builtin: currentProvider()?.builtin || false,
  };
}

async function saveProvider() {
  setBusy(true);
  try {
    const saved = await api.SaveProvider(collectProvider());
    selectedProviderId = saved.id;
    await loadProviders(saved.id);
    setMsg("Provider 已保存。", "ok");
  } catch (e) {
    setMsg("保存 Provider 失败：" + errorText(e), "err");
  } finally {
    setBusy(false);
  }
}

async function verifyProvider() {
  setBusy(true);
  try {
    const saved = await api.SaveProvider(collectProvider());
    selectedProviderId = saved.id;
    const res = await api.VerifyProvider(saved.id);
    await loadProviders(saved.id);
    setMsg(res.ok ? "Provider 验证通过。" : "Provider 验证未通过：" + res.hint, res.ok ? "ok" : "err");
	  } catch (e) {
	    setMsg("验证 Provider 失败：" + errorText(e), "err");
	    await loadProviders(selectedProviderId);
	  } finally {
    setBusy(false);
    await refreshStatus();
  }
}

async function setDefaultProvider() {
  const saved = await api.SaveProvider(collectProvider());
  if (!saved.enabled || !saved.has_key) {
    throw new Error("需先启用并保存 API key，才能设为启动默认。");
  }
  selectedProviderId = saved.id;
  startProviderId = saved.id;
  await api.SetActiveProvider(saved.id);
  await persistSettings();
  await loadProviders(saved.id);
  setMsg("已设为启动默认 provider。", "ok");
}

async function deleteProvider() {
  const p = currentProvider();
  if (!p) return;
  setBusy(true);
  try {
    await api.DeleteProvider(p.id);
    await loadProviders();
    setMsg(p.builtin ? "内置 provider 已停用。" : "Provider 已删除。", "ok");
  } catch (e) {
    setMsg("删除 Provider 失败：" + errorText(e), "err");
  } finally {
    setBusy(false);
  }
}

function addProvider() {
  const id = "custom-" + Date.now().toString().slice(-6);
  const p = providerTemplate(id, "自定义 Provider", "openai_chat_completions", "https://example.com/v1/chat/completions", "model-name", false, false);
  providers.push(p);
  selectedProviderId = id;
  renderProviderList();
  renderProviderEditor(p);
}

function copyProvider() {
  const p = currentProvider();
  if (!p) return addProvider();
  const id = normalizeId(p.id + "-copy");
  const copy = { ...JSON.parse(JSON.stringify(p)), id, display_name: p.display_name + " Copy", builtin: false, active: false, verified: false, api_key: "" };
  providers.push(copy);
  selectedProviderId = id;
  renderProviderList();
  renderProviderEditor(copy);
}

function providerTemplate(id, name, adapter, baseURL, model, enabled, active) {
  return {
    id, display_name: name, adapter, base_url: baseURL, api_key: "", default_model: model,
    models: [{ id: model, display_name: model }],
    model_map: { "claude-opus-4-8": model, "claude-sonnet-5": model, "claude-sonnet-4-6": model, "claude-haiku-4-5": model },
    max_tokens_cap: { [model]: 8192 },
    enabled, builtin: false, verified: false, has_key: false, key_masked: "", active,
  };
}

async function refreshStatus() {
  if (mode === "official") return;
  const s = await api.Status();
  setLight(els.ltProxy, s.proxy);
  setLight(els.ltSandbox, s.sandbox);
  setLight(els.ltUpstream, s.upstream);
  setLight(els.ltPublic, s.public);
}

async function oneClick() {
  if (mode === "official") return runAction("打开官方 Claude Science", () => api.OpenOfficial(), "已打开官方 Claude Science。");
  setBusy(true);
  setMsg("一键开始：选择 provider → 起代理 → 起沙箱 → 探活…");
  try {
    const providerID = els.startProvider.value || startProviderId;
    await api.SetActiveProvider(providerID);
    await persistSettings();
    const r = await api.StartWithProvider(providerID);
    setMsg((r.msg || "已就绪。") + "\n" + (r.url || ""), "ok");
    await loadProviders(providerID);
  } catch (e) {
    setMsg("启动失败：" + errorText(e), "err");
  } finally {
    setBusy(false);
    await refreshStatus();
  }
}

async function stopAll() {
  setBusy(true);
  try {
    await api.StopAll();
    setMsg("已停止代理与沙箱。", "ok");
  } catch (e) {
    setMsg("停止失败：" + errorText(e), "err");
  } finally {
    setBusy(false);
    await refreshStatus();
  }
}

async function refreshLogs() {
  try {
    const logs = await api.ReadLogs();
    setLogOutput(logs);
  } catch (e) {
    setLogOutput("读取日志失败：" + errorText(e));
  }
}

async function clearLogs() {
  await api.ClearLogs();
  await refreshLogs();
}

async function copyPublicURL() {
  const text = els.publicURLPreview.textContent.trim();
  if (!text) throw new Error("公网入口为空");
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const input = document.createElement("input");
  input.value = text;
  input.setAttribute("readonly", "");
  input.style.position = "fixed";
  input.style.opacity = "0";
  document.body.appendChild(input);
  input.select();
  const ok = document.execCommand("copy");
  input.remove();
  if (!ok) throw new Error("剪贴板不可用");
}

function escapeHTML(s) {
  return String(s ?? "").replace(/[&<>"']/g, (ch) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[ch]));
}
function normalizeId(s) {
  return String(s || "").trim().toLowerCase().replace(/[\s_]+/g, "-");
}

window.addEventListener("DOMContentLoaded", async () => {
  api = await loadAPI();
  [
    "panel","pageTitle","pageDesc","modeSeg","startProvider","startProviderSummary","editProviderBtn",
    "providerList","providerEditorTitle","providerStateBadge","providerIdInput","providerNameInput",
    "providerAdapterInput","providerBaseURLInput","providerDefaultModelInput","providerKeyInput",
    "mapOpusInput","mapSonnetInput","mapHaikuInput","providerEnabledInput","addProviderBtn","copyProviderBtn",
    "saveProviderBtn","verifyProviderBtn","setDefaultProviderBtn","deleteProviderBtn",
    "saveNetworkBtn","openPublicBtn","copyPublicURLBtn","proxyPort","sandboxPort","publicPort","publicBaseURL",
    "publicURLPreview","publicRootTarget","publicSandboxTarget","publicRouteSummary","oneClickBtn","stopBtn","msg",
    "logOutput","refreshLogBtn","openLogDirBtn","clearLogBtn","diagnosticOutput","ltProxy","ltSandbox","ltUpstream",
    "ltPublic","openBrowserBtn","doctorBtn","reportBtn","updateBtn","verLabel","aboutVersion","quitBtn",
  ].forEach((id) => (els[id] = $(id)));
  els.proxyStatusValue = els.ltProxy?.parentElement?.querySelector("strong");
  els.sandboxStatusValue = els.ltSandbox?.parentElement?.querySelector("strong");
  els.upstreamStatusValue = els.ltUpstream?.parentElement?.querySelector("strong");
  els.publicStatusValue = els.ltPublic?.parentElement?.querySelector("strong");
  await loadConfig();
  const version = await api.AppVersion();
  els.verLabel.textContent = version;
  if (els.aboutVersion) els.aboutVersion.textContent = version;

  els.startProvider.addEventListener("change", async () => {
    startProviderId = els.startProvider.value;
    selectedProviderId = startProviderId;
    renderProviderList();
    renderProviderEditor(currentProvider());
    updateStartProviderSummary();
    await api.SetActiveProvider(startProviderId);
    await persistSettings();
  });
  els.editProviderBtn.addEventListener("click", () => {
    if (startProviderId) {
      selectedProviderId = startProviderId;
      renderProviderList();
      renderProviderEditor(currentProvider());
    }
    showTab("providerPane");
  });
  [els.proxyPort, els.sandboxPort, els.publicPort, els.publicBaseURL].forEach((el) => {
    el.addEventListener("input", reflectSummary);
    el.addEventListener("change", persistSettings);
  });
  els.addProviderBtn.addEventListener("click", addProvider);
  els.copyProviderBtn.addEventListener("click", copyProvider);
  els.saveProviderBtn.addEventListener("click", saveProvider);
  els.verifyProviderBtn.addEventListener("click", verifyProvider);
  els.setDefaultProviderBtn.addEventListener("click", () => runAction("设为启动默认", setDefaultProvider, ""));
  els.deleteProviderBtn.addEventListener("click", deleteProvider);
  els.saveNetworkBtn.addEventListener("click", () => runAction("保存网络设置", persistSettings, "网络设置已保存。"));
  els.openPublicBtn.addEventListener("click", () => runAction("打开公网入口", () => api.OpenPublicURL(), "已打开公网入口。"));
  els.copyPublicURLBtn.addEventListener("click", () => runAction("复制公网入口", copyPublicURL, "公网入口已复制。"));
  els.refreshLogBtn.addEventListener("click", () => runAction("刷新日志", refreshLogs, "日志已刷新。"));
  els.openLogDirBtn.addEventListener("click", () => runAction("打开日志目录", () => api.OpenLogs(), "已打开日志目录。"));
  els.clearLogBtn.addEventListener("click", () => runAction("清空日志", clearLogs, "日志已清空。"));
  els.oneClickBtn.addEventListener("click", oneClick);
  els.stopBtn.addEventListener("click", stopAll);
  els.openBrowserBtn.addEventListener("click", () => runAction("打开浏览器面板", () => api.OpenURL(), "已打开浏览器面板。"));
  els.doctorBtn.addEventListener("click", () => runAction("自检", () => api.RunDoctor(), (out) => {
    setDiagnostic(out || "自检完成。");
    return "自检完成。";
  }));
  els.reportBtn.addEventListener("click", () => runAction("打开反馈页面", () => api.ReportBug(), "已打开反馈页面。"));
  els.updateBtn.addEventListener("click", () => runAction("检查更新", () => api.OpenReleasePage(), "已打开最新版本页面。"));
  els.quitBtn.addEventListener("click", () => runAction("退出", async () => (await loadRuntime()).Quit(), "正在退出。"));
  document.querySelectorAll(".nav-item").forEach((button) => button.addEventListener("click", () => showTab(button.dataset.tab)));
  els.modeSeg.querySelectorAll(".seg-btn").forEach((b) => b.addEventListener("click", () => runAction("切换模式", async () => {
    await api.SetMode(b.dataset.mode);
    applyMode(b.dataset.mode);
    await refreshStatus();
  }, b.dataset.mode === "official" ? "已切到官方 Claude 模式。" : "已切到第三方模型模式。")));
  await refreshStatus();
});
