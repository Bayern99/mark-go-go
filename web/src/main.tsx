import { createRoot } from 'react-dom/client'
import { isMyGo } from 'mygo-runtime'
import { App } from './App'
import { createBrowserHost } from './host/browserHost'
import { createMyGoHost } from './host/mygoHost'
import './styles.css'

const host = isMyGo() ? createMyGoHost() : createBrowserHost()
const container = document.getElementById('root')
if (!container) throw new Error('#root is missing from index.html')

createRoot(container).render(<App host={host} />)
