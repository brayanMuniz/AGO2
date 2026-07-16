import { Routes, Route, Navigate } from 'react-router-dom';

import SearchPage from './pages/SearchPage';
import ImagePage from './pages/ImagePage';
import StatsPage from './pages/StatsPage';
import SettingsLayout from './layouts/SettingsLayout';
import GeneralSettings from './pages/GeneralSettings';
import DanbooruSettings from './pages/DanbooruSettings';

function App() {
  return (
    <Routes>
      <Route path="/" element={<SearchPage />} />
      <Route path="/image/:id" element={<ImagePage />} />
      <Route path="/stats" element={<StatsPage />} />

      {/* Settings Layout with Nested Routes */}
      <Route path="/settings" element={<SettingsLayout />}>
        <Route index element={<GeneralSettings />} />
        <Route path="danbooru" element={<DanbooruSettings />} />
      </Route>

      <Route path="/search" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

export default App;
