import { Routes, Route, Navigate } from 'react-router-dom';

import SearchPage from './pages/SearchPage';
import ImagePage from './pages/ImagePage';

function App() {
  return (
    <Routes>
      <Route path="/" element={<SearchPage />} />
      <Route path="/image/:id" element={<ImagePage />} />
      <Route path="/search" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

export default App;
