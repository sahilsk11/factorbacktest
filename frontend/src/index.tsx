import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter, Routes, Route } from "react-router-dom";

import './index.css';
import App from './App';
import { BondBuilder } from './Bond/Bond';

const root = ReactDOM.createRoot(
  document.getElementById('root') as HTMLElement
);

root.render(
  <React.StrictMode>
    <BrowserRouter>
      <Routes>
        <Route index element={<App />} />
        <Route path="bonds" element={<BondBuilder />} />
        <Route path="*" element={<p>not found</p>} />
      </Routes>
    </BrowserRouter>

  </React.StrictMode>
);
