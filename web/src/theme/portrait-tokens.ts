/**
 * Shared Portrait light design contract (#25). #26 / #27 consume these names;
 * values live in `styles.css`. Content tokens are not application status.
 */
export const PORTRAIT_FONT_TOKENS = ['--font-sans', '--font-display', '--font-mono'] as const

export const PORTRAIT_TYPE_TOKENS = [
  '--text-display-size',
  '--text-display-leading',
  '--text-title-size',
  '--text-title-leading',
  '--text-reading-size',
  '--text-reading-leading',
  '--text-reading-tracking',
  '--text-ui-size',
  '--text-ui-leading',
  '--text-small-size',
  '--text-small-leading',
  '--text-meta-size',
  '--text-meta-leading',
  '--text-code-size',
  '--text-code-leading',
] as const

export const PORTRAIT_STATUS_TOKENS = [
  '--accent',
  '--warn',
  '--danger',
  '--accent-soft',
  '--warn-soft',
  '--danger-soft',
] as const

export const PORTRAIT_CONTENT_TOKENS = [
  '--content-note',
  '--content-note-soft',
  '--content-warning',
  '--content-warning-soft',
  '--content-caution',
  '--content-caution-soft',
] as const

export const PORTRAIT_MOTION_TOKENS = [
  '--duration-press',
  '--duration-switch',
  '--duration-expand',
  '--duration-enter',
  '--duration-menu',
  '--duration-jump',
  '--ease-out',
] as const

export const PORTRAIT_OFFLINE_FONT_FAMILIES = [
  'IBM Plex Sans',
  'IBM Plex Sans SC',
  'MGG Condensed',
  'Barlow Semi Condensed',
  'IBM Plex Mono',
  'Phosphor',
] as const
