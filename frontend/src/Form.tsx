import { useState } from 'react';
import { FactorData, endpoint } from "./App";
import "./form.css";
import "./app.css";

interface FactorOptions {
  expression: string;
  intensity: number;
  name: string;
}

interface BacktestRequest {
  factorOptions: FactorOptions;
  backtestStart: string;
  backtestEnd: string;
  samplingIntervalUnit: string;
  assetSelectionMode: string;
  startCash: number;
  anchorPortfolioQuantities: Record<string, number>;
  numSymbols?: number;
}

export interface Trade {
  action: string;
  quantity: number;
  symbol: string;
  price: number;
}

export interface BacktestSnapshot {
  valuePercentChange: number;
  value: number;
  date: string;
  assetMetrics: Record<string, SnapshotAssetMetrics>;
}

export interface SnapshotAssetMetrics {
  assetWeight: number;
  factorScore: number;
  priceChangeTilNextResampling?: number | null;
}

interface BacktestResponse {
  factorName: string;
  backtestSnapshots: Record<string, BacktestSnapshot>;
}

export default function FactorForm({
  takenNames,
  appendFactorData
}: {
  takenNames: string[];
  appendFactorData: (newFactorData: FactorData) => void;
}) {
  const [factorOptions, setFactorOptions] = useState({
    expression: `pricePercentChange(
      addDate(currentDate, 0, 0, -7),
      currentDate
) `,
    intensity: 0.75,
    name: "7_day_rolling_price_momentum"
  });
  const [backtestStart, setBacktestStart] = useState("2020-01-02");
  const [backtestEnd, setBacktestEnd] = useState("2022-01-01");
  const [samplingIntervalUnit, setSamplingIntervalUnit] = useState("monthly");
  const [startPortfolio, setStartPortfolio] = useState(`{
      "AAPL": 10,
      "MSFT": 15,
      "GOOGL": 8
}`);
  const [cash, setCash] = useState(10_000);
  const [assetSelectionMode, setAssetSelectionMode] = useState("NUM_SYMBOLS");
  const [numSymbols, setNumSymbols] = useState(10);
  const [names, setNames] = useState<string[]>([...takenNames]);

  let found = false;
  names.forEach(n => {
    if (n === factorOptions.name) {
      found = true;
    }
  })

  const email = document.getElementById("factor-name");
  if (found) {
    (email as HTMLInputElement)?.setCustomValidity("Please use a unique factor name.");
  } else {
    (email as HTMLInputElement)?.setCustomValidity("");
  }

  const handleSubmit = async (e: any) => {
    e.preventDefault();


    const data: BacktestRequest = {
      factorOptions,
      backtestStart,
      backtestEnd,
      samplingIntervalUnit,
      startCash: cash,
      anchorPortfolioQuantities: JSON.parse(startPortfolio),
      assetSelectionMode,
      numSymbols,
    };

    try {
      const response = await fetch(endpoint+"/backtest", {
        method: "POST",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify(data)
      });

      if (response.ok) {
        const result: BacktestResponse = await response.json()
        setNames([...names, factorOptions.name])
        const fd: FactorData = {
          name: data.factorOptions.name,
          data: result.backtestSnapshots,
          expression: data.factorOptions.expression
        } as FactorData;
        appendFactorData(fd)
      } else {
        console.error("Error submitting data:", response.status);
      }
    } catch (error) {
      alert(error)
      console.error("Error:", error);
    }
  };

  // if (found && sendEnabled) {
  //   setSendEnabled(false)
  // } else if (!found && !sendEnabled) {
  //   setSendEnabled(true)
  // }



  return (
    <div className='tile'>
      <h2 style={{ textAlign: "left", margin: "0px" }}>Backtest Strategy</h2>
      <p className='subtext'>Define your quantitative strategy and customize backtest parameters.</p>
      <form onSubmit={handleSubmit}>
        <div className='form-element'>
          <label>Factor Name</label>
          <input style={{width: "250px"}} required
            id="factor-name"
            type="text"
            value={factorOptions.name}
            onChange={(e) =>
              setFactorOptions({ ...factorOptions, name: e.target.value })
            }
          />
        </div>
        <div className='form-element'>
          <label>Factor Expression</label>
          <textarea required
            style={{ height: "150px", width: "250px" }}
            value={factorOptions.expression}
            onChange={(e) =>
              setFactorOptions({ ...factorOptions, expression: e.target.value })
            }
          />
        </div>

        <div className='form-element'>
          <label>Backtest Range</label>
          <input
            required
            type="date"
            value={backtestStart}
            onChange={(e) => setBacktestStart(e.target.value)}
          />
          <p style={{display: "inline"}}> to </p>
          <input
            required
            type="date"
            value={backtestEnd}
            onChange={(e) => setBacktestEnd(e.target.value)}
          />
        </div>
        
        <div className='form-element'>
          <label>Rebalance Interval</label>
          <select value={samplingIntervalUnit} onChange={(e) => setSamplingIntervalUnit(e.target.value)}>
            <option value="daily">daily</option>
            <option value="weekly">weekly</option>
            <option value="monthly">monthly</option>
            <option value="yearly">yearly</option>
          </select>
        </div>

        <div>
          <label>Asset Selection Mode</label>
          <select value={assetSelectionMode} onChange={(e) => setAssetSelectionMode(e.target.value)}>
            <option value="NUM_SYMBOLS">top N scoring assets</option>
            <option value="ANCHOR_PORTFOLIO">tilt existing portfolio</option>
          </select>
        </div>
        {assetSelectionMode === "NUM_SYMBOLS" ? <div>
          <label>Number of Assets</label>
          <input
            type="number"
            value={numSymbols}
            onChange={(e) => setNumSymbols(parseInt(e.target.value))}
          />
        </div> : null}

        {assetSelectionMode === "ANCHOR_PORTFOLIO" ? <div>
          <label>Start Portfolio</label>
          <textarea
            value={startPortfolio}
            onChange={(e) => setStartPortfolio(e.target.value)}
            style={{ height: "100px" }}
          />
        </div> : null}
        <div>
          <label>Starting Cash</label>
          $ <input
            min={1}
            type="number"
            value={cash}
            onChange={(e) => setCash(parseFloat(e.target.value))}
          />
        </div>
        {assetSelectionMode === "ANCHOR_PORTFOLIO" ? <div>
          <label>Intensity</label>
          <input
            type="number"
            value={factorOptions.intensity}
            onChange={(e) =>
              setFactorOptions({ ...factorOptions, intensity: parseFloat(e.target.value) })
            }
          />
        </div> : null}

        <button className='backtest-btn ' type="submit">Run Backtest</button>
      </form>
    </div>
  );
}