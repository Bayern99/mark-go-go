import type { RecentWorkspace } from '../host/types'

export function Welcome({
  recents,
  onOpenFolder,
  onNew,
  onOpenRecent,
}: {
  recents: RecentWorkspace[]
  onOpenFolder: () => void
  onNew: () => void
  onOpenRecent: (root: string) => void
}): React.ReactElement {
  return (
    <div className="welcome" data-testid="welcome">
      <h1>Welcome</h1>
      <p>Open a folder to browse Markdown files, or start an untitled document.</p>
      <div className="welcome-actions">
        <button type="button" className="pill primary" data-testid="open-folder" onClick={onOpenFolder}>
          Open Folder
        </button>
        <button type="button" className="pill" data-testid="new-document" onClick={onNew}>
          New Document
        </button>
      </div>
      <section className="recent-workspaces" aria-label="Recent workspaces">
        <h2>Recent Workspaces</h2>
        {recents.length === 0 ? (
          <p className="nav-empty" data-testid="recent-empty">
            No recent workspaces yet.
          </p>
        ) : (
          <ul data-testid="recent-workspaces">
            {recents.map((item) => (
              <li key={item.root}>
                <button type="button" className="row file" onClick={() => onOpenRecent(item.root)}>
                  <span className="body">
                    <span className="l1">
                      <span className="nm">{item.name}</span>
                    </span>
                    <span className="l2">
                      <span className="s">{item.root}</span>
                    </span>
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  )
}
