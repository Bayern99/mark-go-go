// Chromium reports the real platform; the app's shortcuts follow it.
export const IS_MAC = /Mac/.test(navigator.platform)
export const MOD = IS_MAC ? 'Meta' : 'Control'
