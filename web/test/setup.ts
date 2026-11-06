import { beforeEach } from 'vitest'

// The app remembers the view mode per document path, and every test document
// shares the browser host's path: each test starts from no remembered mode.
beforeEach(() => {
  window.localStorage.clear()
})
