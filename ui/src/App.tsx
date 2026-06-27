import { Routes, Route, Navigate } from 'react-router-dom';

import SearchPage from './pages/SearchPage';
import ImagePage from './pages/ImagePage';
import SettingsLayout from './layouts/SettingsLayout';
import GeneralSettings from './pages/GeneralSettings';
import SyncPage from './pages/SyncPage';
import MatchPage from './pages/MatchPage';
import DuplicatesPage from './pages/DuplicatesPage';

function App() {
  return (
    <Routes>
      <Route path="/" element={<SearchPage />} />
      <Route path="/image/:id" element={<ImagePage />} />

      {/* Settings Layout with Nested Routes */}
      <Route path="/settings" element={<SettingsLayout />}>
        <Route index element={<GeneralSettings />} />
        <Route path="sync" element={<SyncPage />} />
        <Route path="match" element={<MatchPage />} />
        <Route path="duplicates" element={<DuplicatesPage />} />
      </Route>

      <Route path="/search" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

export default App;
