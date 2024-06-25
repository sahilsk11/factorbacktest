import { Dispatch, SetStateAction, useEffect, useState } from 'react';
import { FactorData, endpoint } from "./App";
import "./form.css";
import "./app.css";
import { BacktestRequest, BacktestResponse, FactorOptions } from './models';


export default function FactorForm({
  userID,
  takenNames,
  appendFactorData
}: {
  userID: string,
  takenNames: string[];
  appendFactorData: (newFactorData: FactorData) => void;
}) {
  const [factorExpression, setFactorExpression] = useState("");

  const [factorIntensity, setFactorIntensity] = useState(0.75);

  const [factorName, setFactorName] = useState("7_day_rolling_price_momentum");

  const [backtestStart, setBacktestStart] = useState("2020-01-01");
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
    if (n === factorName) {
      found = true;
    }
  })

  const factorNameInput = document.getElementById("factor-name");
  if (found) {
    (factorNameInput as HTMLInputElement)?.setCustomValidity("Please use a unique factor name.");
  } else {
    (factorNameInput as HTMLInputElement)?.setCustomValidity("");
  }
  const cashInput = document.getElementById("cash");
  if (cash <= 0) {
    (cashInput as HTMLInputElement)?.setCustomValidity("Please enter more than $0.")
  } else {
    (cashInput as HTMLInputElement)?.setCustomValidity("")
  }
  const numSymbolsInput = document.getElementById("num-symbols");
  if (numSymbols <= 0) {
    (numSymbolsInput as HTMLInputElement)?.setCustomValidity("Please enter more than 0 assets.")
  } else if (numSymbols > 100) {
    (numSymbolsInput as HTMLInputElement)?.setCustomValidity("Please use less than 100 assets.")
  } else {
    (numSymbolsInput as HTMLInputElement)?.setCustomValidity("")
  }

  const handleSubmit = async (e: any) => {
    e.preventDefault();
    setErr(null);
    setLoading(true);

    const data: BacktestRequest = {
      factorOptions: {
        expression: factorExpression,
        name: factorName,
        intensity: factorIntensity,
      } as FactorOptions,
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
        setNames([...names, factorName])
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
            value={factorName}
            onChange={(e) =>
              setFactorName(e.target.value)
            }
          />
        </div>
        <div className='form-element'>
          <FactorExpressionInput
            userID={userID}
            factorExpression={factorExpression}
            setFactorExpression={setFactorExpression}
            setFactorName={setFactorName}
          />
        </div>

        <div className='form-element'>
          <label>Backtest Range</label>
          <input
            min={'2018-01-01'}
            max={backtestEnd > '2023-09-05' ? '2023-09-05' : backtestEnd}
            required
            type="date"
            value={backtestStart}
            onChange={(e) => setBacktestStart(e.target.value)}
          />
          <p style={{ display: "inline" }}> to </p>
          <input
            max={'2023-09-05'}
            required
            type="date"
            value={backtestEnd}
            onChange={(e) => setBacktestEnd(e.target.value)}
          />
        </div>

        <div className='form-element'>
          <label>Rebalance Interval</label>
          <p className='label-subtext'>How frequently should we re-evaluate portfolio holdings.</p>
          <select value={samplingIntervalUnit} onChange={(e) => setSamplingIntervalUnit(e.target.value)}>
            <option value="daily">daily</option>
            <option value="weekly">weekly</option>
            <option value="monthly">monthly</option>
            <option value="yearly">yearly</option>
          </select>
        </div>

        <div style={{ display: "none" }}>
          <label>Asset Selection Mode</label>
          <select value={assetSelectionMode} onChange={(e) => setAssetSelectionMode(e.target.value)}>
            <option value="NUM_SYMBOLS">top N scoring assets</option>
            <option value="ANCHOR_PORTFOLIO">tilt existing portfolio</option>
          </select>
        </div>
        {assetSelectionMode === "NUM_SYMBOLS" ? <div>
          <label>Number of Assets</label>
          <p className='label-subtext'>How many assets the target portfolio should hold at any time.</p>
          <input
            id="num-symbols"
            max={100}
            style={{ width: "80px" }}
            value={numSymbols}
            onChange={(e) => {
              let x = e.target.value;
              if (x.length === 0) {
                x = "0";
              }
              if (!/[^0-9]/.test(x)) {
                setNumSymbols(parseFloat(x))
              }
            }
            }
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
          <span style={{fontSize: "14px"}}>$</span> <input
            id="cash"
            value={cash.toLocaleString()}
            style={{ paddingLeft: "5px" }}
            onChange={(e) => {
              let x = e.target.value.replace(/,/g, '')
              if (x.length === 0) {
                x = "0";
              }
              if (!/[^0-9]/.test(x) && x.length < 12) {
                setCash(parseFloat(x))
              }
            }}
          />
        </div>
        <div className='form-element'>
          <label>Asset Universe</label>
          <p className='label-subtext'>The pool of assets that are eligible for the target portfolio.</p>
          <select>
            <option value="daily">SPY Top 80 Holdings</option>
          </select>
        </div> 
        {assetSelectionMode === "ANCHOR_PORTFOLIO" ? <div>
          <label>Intensity</label>
          <input
            type="number"
            value={factorIntensity}
            onChange={(e) =>
              setFactorIntensity(parseFloat(e.target.value))
            }
          />
        </div> : null}

        {loading ? <img style={{ width: "40px", marginTop: "20px", marginLeft: "40px" }} src='loading.gif' /> : <button className='backtest-btn ' type="submit">Run Backtest</button>}

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

function FactorExpressionInput({ userID, factorExpression, setFactorExpression, setFactorName }: {
  userID: string;
  factorExpression: string;
  setFactorExpression: Dispatch<SetStateAction<string>>;
  setFactorName: Dispatch<SetStateAction<string>>;
}) {
  const [gptInput, setGptInput] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [selectedFactor, setSelectedFactor] = useState("momentum");

  interface selectedFactorDetails {
    expression: string;
    factorName: string;
  }

  const presetMap: Record<string, selectedFactorDetails> = {
    "gpt": {
      expression: "",
      factorName: ""
    },
    "momentum": {
      expression: `pricePercentChange(
  nDaysAgo(7),
  currentDate
) `,
      factorName: "7_day_momentum"
    },
    "value": {
      expression: "1/pbRatio(currentDate)",
      factorName: "undervalued_by_pb_ratio"
    },
    "volatility": {
      expression: "1/stdev(nYearsAgo(1), currentDate)",
      factorName: "low_volatility"
    },
    "size": {
      expression: "1/marketCap(currentDate)",
      factorName: "small_cap"
    },
  }

  useEffect(() => {
    setFactorExpression(presetMap[selectedFactor].expression)
    if (selectedFactor !== "gpt") {
      setFactorName(presetMap[selectedFactor].factorName)
    }
  }, [selectedFactor])

  const autofillEquation = async (e: any) => {
    e.preventDefault();
    setLoading(true);
    try {
      const response = await fetch(endpoint + "/constructFactorEquation", {
        method: "POST",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify({ input: gptInput, userID })
      });
      setLoading(false);
      if (response.ok) {
        const result = await response.json()
        if (result.error.length == 0) {
          setFactorExpression(result.factorExpression)
          setFactorName(result.factorName)
        } else {
          setErr(result.error + " - " + result.reason);
        }
      } else {
        const j = await response.json()
        setErr(j.error + " - " + j.reason);
        console.error("Error submitting data:", response.status);
      }
    } catch (error) {
      setLoading(false)
      setErr((error as Error).message)
      console.error("Error:", error);
    }
  }

  return <>
    <div>
      <label>Factor Expression</label>
      <p className='label-subtext'>Higher scoring assets will have a larger allocation in the portfolio.</p>

      <select
        onChange={(e) => setSelectedFactor(e.target.value)}
        style={{ fontSize: "14px" }}
      >
        <option value="momentum">Momentum (price trending up)</option>
        <option value="value">Value (undervalued relative to price)</option>
        <option value="size">Size (smaller assets by market cap)</option>
        <option value="volatility">Volatility (low risk assets)</option>
        <option value="gpt">Describe factor in English (ChatGPT)</option>
      </select>
      {selectedFactor === "gpt" ? <>
        <p style={{ marginTop: "5px" }} className='label-subtext'>Uses ChatGPT API to convert factor description to equation.</p>
        <div className='gpt-input-wrapper'>
          <textarea
            style={{
              width: "250px",
              height: "38px",
              fontSize: "14px"
            }}
            placeholder='small cap, undervalued, and price going up'
            value={gptInput}
            onChange={(e) => setGptInput(e.target.value)}
          />
          <button className='gpt-submit' onClick={(e) => autofillEquation(e)}>➜</button>
        </div>
      </> : null}

      {selectedFactor === "gpt" && gptInput.length > 0 ?
        <p style={{ marginTop: "5px", maxWidth: "300px" }} className='label-subtext'>ChatGPT may determine incorrect equations. Be sure to double check and modify if necessary.</p>
        :
        <p style={{ marginTop: "5px" }} className='label-subtext'>Or enter equation manually.</p>}
      <textarea required
        style={{ height: "80px", width: "250px", fontSize: "13px" }}
        value={factorExpression}
        onChange={(e) =>
          setFactorExpression(e.target.value)
        }
      />
    </div>
  </>
}