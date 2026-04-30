import { Route, Routes } from 'react-router';

import { RootLayout } from '@/components/layout/RootLayout';
import { BacktestPage } from '@/pages/Backtest/BacktestPage';
import { BuilderPage } from '@/pages/Builder/BuilderPage';
import { HomePage } from '@/pages/Home/HomePage';
import { InvestmentsPage } from '@/pages/Investments/InvestmentsPage';

// Route table. Add new pages here, not in main.tsx.
function App() {
  return (
    <Routes>
      <Route element={<RootLayout />}>
        <Route index element={<HomePage />} />
        <Route path="builder" element={<BuilderPage />} />
        <Route path="backtest" element={<BacktestPage />} />
        <Route path="investments" element={<InvestmentsPage />} />
      </Route>
    </Routes>
  );
}

export default App;
