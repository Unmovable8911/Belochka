import { BrowserRouter, Routes, Route } from "react-router-dom"
import Dashboard from "./pages/Dashboard"
import ServerDetail from "./pages/ServerDetail"

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/server/:id" element={<ServerDetail />} />
      </Routes>
    </BrowserRouter>
  )
}

export default App
