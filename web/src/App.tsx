import { BrowserRouter, Routes, Route } from "react-router-dom"
import { WebSocketProvider } from "./components/WebSocketProvider"
import Dashboard from "./pages/Dashboard"
import ServerDetail from "./pages/ServerDetail"

function App() {
  return (
    <BrowserRouter>
      <WebSocketProvider>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/server/:id" element={<ServerDetail />} />
        </Routes>
      </WebSocketProvider>
    </BrowserRouter>
  )
}

export default App
