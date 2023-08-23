import { useEffect, useState } from 'react';

import './app.css'

import {
  BacktestSample,
  BacktestResponse,
  BenchmarkData,
  DatasetInfo
} from './models';

import BacktestChart from './BacktestChart';

import Form from "./Form";
import BenchmarkManager from './BenchmarkSelector';


const App = () => {
  const [results, updateResults] = useState<BacktestResponse[]>([]);
  const [selectedBenchmarks, updateSelectedBenchmarks] = useState(["SPY"]);

  const [benchmarkData, updateBenchmarkData] = useState<BenchmarkData[]>([]);




  useEffect(() => {
    const fetchData = async (symbol: string): Promise<BenchmarkData | null> => {
      try {
        const response = await fetch(
          'http://localhost:3009/benchmark',
          {
            method: "POST",
            headers: {
              "Content-Type": "application/json"
            },
            body: JSON.stringify({
              symbol,
              start: "2018-01-01",
              end: "2023-01-01",
              granularity: "monthly"
            }),
          }
        );
        const d = await response.json();

        return {
          symbol,
          data: d,
        } as BenchmarkData;
      } catch (error) {
        console.error('Error fetching data:', error);
      }
      return null;
    };

    const wrapper = async () => {
      let newBenchmarkData: BenchmarkData[] = [];

      await Promise.all(selectedBenchmarks.map(async b => {
        const newData = await fetchData(b);
        if (newData !== null) {
          newBenchmarkData.push(newData)
        }
        return newData
      }))
      updateBenchmarkData(newBenchmarkData)
    }

    wrapper()
  }, [results, selectedBenchmarks])









  return <>
    <div className="centered-container">
      <div className="container">
        <div className="column" style={{ "flexGrow": 2 }}>
          <Form selectedBenchmarks={selectedBenchmarks} results={results} updateResults={updateResults} />
        </div>
        <div className="column" style={{ "flexGrow": 4 }}>
          <BacktestChart
            benchmarkData={benchmarkData}
            results={results}
          />
        </div>
      </div>
    </div>
    <BenchmarkManager selectedBenchmarks={selectedBenchmarks} updateSelectedBenchmarks={updateSelectedBenchmarks} />
  </>
}

export default App;





