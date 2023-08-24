import { useState } from 'react';
import { FactorData } from "./App";

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

interface Trade {
  action: string;
  quantity: number;
  symbol: string;
  price: number;
}

interface AssetWeights {
  [symbol: string]: number;
}

export interface BacktestSnapshot {
  valuePercentChange: number;
  value: number;
  date: string;
  assetWeights: AssetWeights;
  trades: Trade[];
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
    expression: `pricePercentChange(addDate(currentDate, 0, 0, -7),currentDate) `,
    intensity: 0.75,
    name: "test"
  });
  const [backtestStart, setBacktestStart] = useState("2020-01-02");
  const [backtestEnd, setBacktestEnd] = useState("2020-02-01");
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
  const [sendEnabled, setSendEnabled] = useState(true);

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
      const response = await fetch("http://localhost:3009/backtest", {
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
        // Object.keys(result.backtestSnapshots).forEach(date => {
        //   const value = result.backtestSnapshots[date];
        //   fd.data[date] = {
        //     totalValue: value.value,
        //     percentChange: value.valuePercentChange,
        //     date,
        //   } as Portfolio
        // })
        appendFactorData(fd)
      } else {
        console.error("Error submitting data:", response.status);
      }
    } catch (error) {
      alert(error)
      console.error("Error:", error);
    }
  };

  let found = false;
  names.forEach(n => {
    if (n === factorOptions.name) {
      found = true;
    }
  })
  if (found && sendEnabled) {
    setSendEnabled(false)
  } else if (!found && !sendEnabled) {
    setSendEnabled(true)
  }


  return (
    <div>
      <form onSubmit={handleSubmit}>
        <div>
          <label>Factor Expression:</label>
          <br />
          <textarea
            style={{ height: "100px" }}
            value={factorOptions.expression}
            onChange={(e) =>
              setFactorOptions({ ...factorOptions, expression: e.target.value })
            }
          />
        </div>
        {found ? <p>this name is already used</p> : null}
        <div>
          <label>Factor Name:</label>
          <input
            type="text"
            value={factorOptions.name}
            onChange={(e) =>
              setFactorOptions({ ...factorOptions, name: e.target.value })
            }
          />
        </div>
        <div>
          <label>Backtest Start:</label>
          <input
            type="date"
            value={backtestStart}
            onChange={(e) => setBacktestStart(e.target.value)}
          />
        </div>
        <div>
          <label>Backtest End:</label>
          <input
            type="date"
            value={backtestEnd}
            onChange={(e) => setBacktestEnd(e.target.value)}
          />
        </div>
        <div>
          <label>Sampling Interval Unit:</label>
          <input
            type="text"
            value={samplingIntervalUnit}
            onChange={(e) => setSamplingIntervalUnit(e.target.value)}
          />
        </div>

        <div>
          <label>Asset Selection Mode:</label>
          <input
            type="text"
            value={assetSelectionMode}
            onChange={(e) => setAssetSelectionMode(e.target.value)}
          />
        </div>
        {assetSelectionMode === "NUM_SYMBOLS" ? <div>
          <label>Num Symbols:</label>
          <input
            type="number"
            value={numSymbols}
            onChange={(e) => setNumSymbols(parseInt(e.target.value))}
          />
        </div> : null}

        {assetSelectionMode === "ANCHOR_PORTFOLIO" ? <div>
          <label>Start Portfolio:</label>
          <br />
          <textarea
            value={startPortfolio}
            onChange={(e) => setStartPortfolio(e.target.value)}
            style={{ height: "100px" }}
          />
        </div> : null}
        <div>
          <label>Cash:</label>
          <input
            type="number"
            value={cash}
            onChange={(e) => setCash(parseFloat(e.target.value))}
          />
        </div>
        {assetSelectionMode === "ANCHOR_PORTFOLIO" ? <div>
          <label>Intensity:</label>
          <input
            type="number"
            value={factorOptions.intensity}
            onChange={(e) =>
              setFactorOptions({ ...factorOptions, intensity: parseFloat(e.target.value) })
            }
          />
        </div> : null}
        
        <button disabled={!sendEnabled} type="submit">Submit</button>
      </form>
    </div>
  );
}