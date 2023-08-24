import { useEffect, useState } from 'react';

import './app.css'

import BacktestChart from './BacktestChart';

import FactorForm from "./Form";
import BenchmarkManager from './BenchmarkSelector';

import { minMaxDates } from './util';

export interface Portfolio {
  totalValue: number,
  percentChange: number,
  holdingsWeight: Record<string, number>,
  cash: number,
  date: string
}
export interface FactorData {
  name: string,
  expression: string,
  // options
  data: Record<string, Portfolio>,
}

export interface BenchmarkData {
  symbol: string,
  data: Record<string, number>
}

const App = () => {
  const [factorData, updateFactorData] = useState<FactorData[]>([]);
  const [benchmarkData, updateBenchmarkData] = useState<BenchmarkData[]>([]);

  let takenNames: string[] = [];
  factorData.forEach(fd => {
    takenNames.push(fd.name)
  })
  benchmarkData.forEach(bd => {
    takenNames.push(bd.symbol)
  })
  const { min: minFactorDate, max: maxFactorDate } = minMaxDates(factorData);
  console.log(factorData)
  console.log(minFactorDate, maxFactorDate)
  return <>
    <div className="centered-container">
      <div className="container">
        <div className="column" style={{ "flexGrow": 2 }}>
          <FactorForm
            // set this to the benchmark names that are already in used
            takenNames={takenNames}
            appendFactorData={(newFactorData: FactorData) => {
              updateFactorData([...factorData, newFactorData])
            }}
          />
        </div>
        <div className="column" style={{ "flexGrow": 4 }}>
          <BacktestChart
            benchmarkData={benchmarkData}
            factorData={factorData}
          />
        </div>
      </div>
    </div>
    <BenchmarkManager
      minDate={minFactorDate}
      maxDate={maxFactorDate}
      updateBenchmarkData={updateBenchmarkData}
    />
  </>
}

export default App;





