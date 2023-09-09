import { useState } from 'react';
import { FactorData, endpoint } from "./App";
import "./form.css";
import "./app.css";
import { BacktestRequest, BacktestResponse } from './models';


export default function FactorForm({
  userID,
  takenNames,
  appendFactorData
}: {
  userID: string,
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
  const [err, setErr] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

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
    setErr(null);
    setLoading(true);

    const data: BacktestRequest = {
      factorOptions,
      backtestStart,
      backtestEnd,
      samplingIntervalUnit,
      startCash: cash,
      anchorPortfolioQuantities: JSON.parse(startPortfolio),
      assetSelectionMode,
      numSymbols,
      userID
    };

    try {
      const response = await fetch(endpoint + "/backtest", {
        method: "POST",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify(data)
      });
      setLoading(false);
      if (response.ok) {
        const result: BacktestResponse = await response.json()
        if (Object.keys(result.backtestSnapshots).length === 0) {
          setErr("No backtest results were calculated");
          return;
        }
        setNames([...names, factorOptions.name])
        const fd: FactorData = {
          name: data.factorOptions.name,
          data: result.backtestSnapshots,
          expression: data.factorOptions.expression
        } as FactorData;
        appendFactorData(fd)
      } else {
        const j = await response.json()
        setErr(j.error)
        console.error("Error submitting data:", response.status);
      }
    } catch (error) {
      setLoading(false)
      setErr((error as Error).message)
      console.error("Error:", error);
    }
  };

  return (
    <div className='tile'>
      <h2 style={{ textAlign: "left", margin: "0px" }}>Backtest Strategy</h2>
      <p className='subtext'>Define your quantitative strategy and customize backtest parameters.</p>
      <form onSubmit={handleSubmit}>
        <div className='form-element'>
          <label>Factor Name</label>
          <input style={{ width: "250px" }} required
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
          <p style={{ display: "inline" }}> to </p>
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

        {loading ? <img style={{width: "40px", marginTop: "20px", marginLeft: "50px"}} src='loading.gif' /> : <button className='backtest-btn ' type="submit">Run Backtest</button> }

        <Error message={err} />
      </form>
    </div>
  );
}

function Error({ message }: { message: string | null }) {
  return message === null ? null : <>
    <div className='error-container'>
      <h4 style={{ marginBottom: "0px", marginTop: "0px" }}>That's an error.</h4>
      <p>{message}</p>
    </div>
  </>
}