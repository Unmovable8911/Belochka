import { BrowserRouter, Routes, Route } from "react-router-dom"
import { WebSocketProvider } from "./components/WebSocketProvider"
import { ConnectionBanner } from "./components/ConnectionBanner"
import { StaleDataOverlay } from "./components/StaleDataOverlay"
import { Layout } from "./components/Layout"
import Dashboard from "./pages/Dashboard"
import ServerDetail from "./pages/ServerDetail"

function App() {
  return (
    <BrowserRouter>
      <WebSocketProvider>
        <Layout>
          <ConnectionBanner />
          <StaleDataOverlay>
            <Routes>
              <Route path="/" element={<Dashboard />} />
              <Route path="/server/:id" element={<ServerDetail />} />
            </Routes>
          </StaleDataOverlay>
        </Layout>
      </WebSocketProvider>
    </BrowserRouter>
  )
}

export default App
