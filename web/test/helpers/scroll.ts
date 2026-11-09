export function scrollViewport(host: Element): HTMLElement {
  const viewport = host.matches('[data-overlayscrollbars-viewport]')
    ? host
    : host.querySelector('[data-overlayscrollbars-viewport]')
  if (!(viewport instanceof HTMLElement)) throw new Error('overlay scroll viewport missing')
  return viewport
}
