// This file is the Wails binding wrapper for the Go App object. It mirrors
// exactly the shape Wails would generate; it is maintained manually because
// the generator relies on an older x/tools that cannot type-load Go 1.27
// standard packages.
//
// IMPORTANT: the bridge (window.go) is injected by Wails at runtime, so it is
// resolved lazily inside each call. Accessing window.go at module load time
// would throw "Cannot read properties of undefined (reading 'main')".

// @ts-check

function call(method, args) {
  const go = window['go'];
  if (!go || !go['main'] || !go['main']['App'] || typeof go['main']['App'][method] !== 'function') {
    throw new Error(
      'El puente con Karina todavía no está disponible. Abre la aplicación de escritorio (Karina.exe).',
    );
  }
  return go['main']['App'][method].apply(null, args);
}

export function GetAppInfo() {
  return call('GetAppInfo', []);
}

export function CheckForUpdate() {
  return call('CheckForUpdate', []);
}

export function RefreshNow() {
  return call('RefreshNow', []);
}

export function ListProviders() {
  return call('ListProviders', []);
}

export function States() {
  return call('States', []);
}

export function Config() {
  return call('Config', []);
}

export function KeyPreview(provider) {
  return call('KeyPreview', [provider]);
}

export function TestProvider(provider, key) {
  return call('TestProvider', [provider, key]);
}

export function SaveProviderKey(provider, key) {
  return call('SaveProviderKey', [provider, key]);
}

export function RemoveProvider(provider) {
  return call('RemoveProvider', [provider]);
}

export function SetProviderEnabled(provider, enabled) {
  return call('SetProviderEnabled', [provider, enabled]);
}

export function SetManualUsage(provider, used, limit, window) {
  return call('SetManualUsage', [provider, used, limit, window]);
}

export function ExperimentalClaudeSubscription(token) {
  return call('ExperimentalClaudeSubscription', [token]);
}

export function ClaudeOAuthStart() {
  return call('ClaudeOAuthStart', []);
}

export function ClaudeOAuthComplete(code) {
  return call('ClaudeOAuthComplete', [code]);
}

export function LogClientError(message, stack) {
  return call('LogClientError', [message, stack]);
}

export function SetRefreshInterval(seconds) {
  return call('SetRefreshInterval', [seconds]);
}

export function SetStartWithSystem(enabled) {
  return call('SetStartWithSystem', [enabled]);
}

export function SetAlertsEnabled(enabled) {
  return call('SetAlertsEnabled', [enabled]);
}

export function SetAlertThreshold(percent) {
  return call('SetAlertThreshold', [percent]);
}

export function CompleteOnboarding() {
  return call('CompleteOnboarding', []);
}

export function EnterWidgetMode() {
  return call('EnterWidgetMode', []);
}

export function ExitWidgetMode() {
  return call('ExitWidgetMode', []);
}

export function History(provider, span) {
  return call('History', [provider, span]);
}

export function ExportHistory(provider, span) {
  return call('ExportHistory', [provider, span]);
}
