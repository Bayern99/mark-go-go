import { createRoot } from 'react-dom/client'
import { isMyGo } from 'mygo-runtime'
import { App } from './App'
import { createBrowserHost, createBrowserWorkspace } from './host/browserHost'
import { createMyGoHost } from './host/mygoHost'
import './assets/fonts.css'
import './assets/icons/phosphor.css'
import './styles.css'

const workspace = isMyGo() ? createMyGoHost() : createBrowserWorkspace([createBrowserHost()])
const container = document.getElementById('root')
if (!container) throw new Error('#root is missing from index.html')

createRoot(container).render(<App workspace={workspace} />)
