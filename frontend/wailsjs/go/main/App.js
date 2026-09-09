// This file is the Wails binding wrapper for the Go App object. It mirrors
// exactly the shape Wails would generate; it is maintained manually because
// the generator relies on an older x/tools that cannot type-load Go 1.27
// standard packages. All calls go through the injected window.go bridge.

// @ts-check

export function GetAppInfo() {
  return window['go']['main']['App']['GetAppInfo']();
}

export function RefreshNow() {
  return window['go']['main']['App']['RefreshNow']();
}

export function ListProviders() {
  return window['go']['main']['App']['ListProviders']();
}

export function States() {
  return window['go']['main']['App']['States']();
}

export function Config() {
  return window['go']['main']['App']['Config']();
}

export function KeyPreview(provider) {
  return window['go']['main']['App']['KeyPreview'](provider);
}

export function TestProvider(provider, key) {
  return window['go']['main']['App']['TestProvider'](provider, key);
}

export function SaveProviderKey(provider, key) {
  return window['go']['main']['App']['SaveProviderKey'](provider, key);
}

export function RemoveProvider(provider) {
  return window['go']['main']['App']['RemoveProvider'](provider);
}

export function SetProviderEnabled(provider, enabled) {
  return window['go']['main']['App']['SetProviderEnabled'](provider, enabled);
}

export function SetRefreshInterval(seconds) {
  return window['go']['main']['App']['SetRefreshInterval'](seconds);
}

export function SetStartWithSystem(enabled) {
  return window['go']['main']['App']['SetStartWithSystem'](enabled);
}

export function CompleteOnboarding() {
  return window['go']['main']['App']['CompleteOnboarding']();
}

export function EnterWidgetMode() {
  return window['go']['main']['App']['EnterWidgetMode']();
}

export function ExitWidgetMode() {
  return window['go']['main']['App']['ExitWidgetMode']();
}

export function History(provider, span) {
  return window['go']['main']['App']['History'](provider, span);
}
