import { Route, Routes } from 'react-router';

import { RootLayout } from '@/components/layout/RootLayout';
import { ComingSoonPage } from '@/pages/ComingSoon';
import { HomePage } from '@/pages/Home/HomePage';

// Route table. Add new pages here, not in main.tsx.
function App() {
  return (
    <Routes>
      <Route element={<RootLayout />}>
        <Route index element={<HomePage />} />
        <Route path="backtest" element={<ComingSoonPage pageName="Backtest" />} />
        <Route path="investments" element={<ComingSoonPage pageName="Investments" />} />
      </Route>
    </Routes>
  );
}

export default App;
