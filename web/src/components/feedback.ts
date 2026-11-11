import { toast } from 'sonner'

/** Copy feedback is momentary and shares one slot instead of stacking. */
export const COPY_TOAST_ID = 'mgo-copy'

export function notifyCopied(kind: 'Code' | 'Link'): void {
  toast.success(`${kind} copied`, { id: COPY_TOAST_ID, testId: 'copy-toast' })
}
