import { useState, useEffect } from "react"
import { BrowserRouter, Routes, Route } from "react-router-dom"
import { WebSocketProvider } from "./components/WebSocketProvider"
import { ConnectionBanner } from "./components/ConnectionBanner"
import { StaleDataOverlay } from "./components/StaleDataOverlay"
import { Layout } from "./components/Layout"
import Dashboard from "./pages/Dashboard"
import ServerDetail from "./pages/ServerDetail"
import Console from "./pages/Console"
import SetupPage from "./pages/SetupPage"
import LoginPage from "./pages/LoginPage"
import * as api from "@/api/client"

type AppState = "loading" | "setup" | "login" | "app"

function App() {
  const [state, setState] = useState<AppState>("loading")

  useEffect(() => {
    api.getAuthStatus().then((s) => {
      if (s.needs_setup) setState("setup")
      else if (!s.authenticated) setState("login")
      else setState("app")
    }).catch(() => setState("login"))
  }, [])

  if (state === "loading") return null

  return (
    <BrowserRouter>
      {state === "setup" ? (
        <Routes>
          <Route path="*" element={<SetupPage />} />
        </Routes>
      ) : state === "login" ? (
        <Routes>
          <Route path="*" element={<LoginPage />} />
        </Routes>
      ) : (
        <Routes>
          <Route path="/server/:id/console" element={<Console />} />
          <Route path="*" element={
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
          } />
        </Routes>
      )}
    </BrowserRouter>
  )
}

export default App
