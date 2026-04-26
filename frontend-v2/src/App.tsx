import { Route, Routes } from 'react-router';

import { RootLayout } from '@/components/layout/RootLayout';
import { BacktestPage } from '@/pages/Backtest/BacktestPage';
import { BuilderPage } from '@/pages/Builder/BuilderPage';
import { ComingSoonPage } from '@/pages/ComingSoon';
import { HomePage } from '@/pages/Home/HomePage';

// Route table. Add new pages here, not in main.tsx.
function App() {
  return (
    <Routes>
      <Route element={<RootLayout />}>
        <Route index element={<HomePage />} />
        <Route path="builder" element={<BuilderPage />} />
        <Route path="backtest" element={<BacktestPage />} />
        <Route path="investments" element={<ComingSoonPage pageName="Investments" />} />
      </Route>
    </Routes>
  );
}

export default App;
