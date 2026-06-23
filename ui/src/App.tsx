import './App.css'
import { Routes, Route } from "react-router-dom";

import SearchPage from "./pages/SearchPage";
import ImagePage from "./pages/ImagePage";

function App() {

  return (
    <>
      <Routes>
        <Route path="/image/:id" element={<ImagePage />} />
        <Route path="/search" element={<SearchPage />} />
      </Routes>

    </>
  )
}

export default App
