import { useEffect, useState } from 'react';
import './app.css'
import BacktestChart from './BacktestChart';
import FactorForm from "./Form";
import InspectFactorData from './FactorSnapshot';
import BenchmarkManager from './BenchmarkSelector';
import { BacktestSnapshot } from "./models";
import { minMaxDates } from './util';
import {v4 as uuidv4} from 'uuid';


export interface FactorData {
  name: string,
  expression: string,
  // options
  data: Record<string, BacktestSnapshot>,
}

export const endpoint = (process.env.NODE_ENV === 'production') ? "https://api.factorbacktest.net" : "http://localhost:3009";

export interface BenchmarkData {
  symbol: string,
  data: Record<string, number>
}

const App = () => {
  const [userID, setUserID] = useState("");
  const [factorData, updateFactorData] = useState<FactorData[]>([]);
  const [benchmarkData, updateBenchmarkData] = useState<BenchmarkData[]>([]);
  const [inspectFactorDataIndex, updateInspectFactorDataIndex] = useState<number | null>(null);
  const [inspectFactorDataDate, updateInspectFactorDataDate] = useState<string | null>(null);

  useEffect(() => {
    const queryString = window.location.search;
    const urlParams = new URLSearchParams(queryString);
    setUserID(urlParams.get('userID') || urlParams.get('id') || uuidv4())
  }, []);

  let takenNames: string[] = [];
  factorData.forEach(fd => {
    takenNames.push(fd.name)
  })
  benchmarkData.forEach(bd => {
    takenNames.push(bd.symbol)
  })
  const { min: minFactorDate, max: maxFactorDate } = minMaxDates(factorData);

  useEffect(() => {
    if (factorData.length > 0) {
      updateInspectFactorDataIndex(factorData.length - 1);
      const d = factorData[factorData.length - 1].data;
      const key = Object.keys(d).reduce((a, b) => (a > b ? b : a));
      updateInspectFactorDataDate(key);
    }
  }, [factorData])

  return <>

    <div className="centered-container">
      <div className="container">
        <div className="column" style={{ "flexGrow": 2, marginRight: "20px" }}>
          <FactorForm
            // set this to the benchmark names that are already in used
            userID={userID}
            takenNames={takenNames}
            appendFactorData={(newFactorData: FactorData) => {
              updateFactorData([...factorData, newFactorData])
            }}
          />
          <BenchmarkManager
            minDate={minFactorDate}
            maxDate={maxFactorDate}
            updateBenchmarkData={updateBenchmarkData}
          />
        </div>
        <div className="column" style={{ "flexGrow": 4 }}>
          <BacktestChart
            benchmarkData={benchmarkData}
            factorData={factorData}
            updateInspectFactorDataIndex={updateInspectFactorDataIndex}
            updateInspectFactorDataDate={updateInspectFactorDataDate}
          />
          <InspectFactorData
            fdIndex={inspectFactorDataIndex}
            fdDate={inspectFactorDataDate}
            factorData={factorData}
          />
        </div>
      </div>
    </div>
    <div style={{ height: "100px" }}></div>
  </>
}

export default App;





