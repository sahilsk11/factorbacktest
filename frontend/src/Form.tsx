import { Dispatch, SetStateAction, useEffect, useState } from 'react';
import { FactorData, endpoint } from "./App";
import "./form.css";
import "./app.css";
import { BacktestRequest, GetAssetUniversesResponse, BacktestResponse, FactorOptions } from './models';
import 'react-tooltip/dist/react-tooltip.css'
import { daysBetweenDates } from './util';
import { FactorExpressionInput } from './FactorExpressionInput';

function todayAsString() {
  const today = new Date();
  const year = today.getFullYear();
  const month = String(today.getMonth() + 1).padStart(2, '0'); // Months are 0-based, so add 1
  const day = String(today.getDate()).padStart(2, '0');

  return `${year}-${month}-${day}`;
}

function twoYearsAgoAsString() {
  const today = new Date();
  const year = today.getFullYear() - 2;
  const month = String(today.getMonth() + 1).padStart(2, '0'); // Months are 0-based, so add 1
  const day = String(today.getDate()).padStart(2, '0');

  return `${year}-${month}-${day}`;
}

export default function FactorForm({
  userID,
  takenNames,
  appendFactorData,
  fullscreenView,
}: {
  userID: string,
  takenNames: string[];
  appendFactorData: (newFactorData: FactorData) => void;
  fullscreenView: boolean,
}) {
  const [factorExpression, setFactorExpression] = useState("");
  const [factorName, setFactorName] = useState("7_day_momentum_weekly");
  const [backtestStart, setBacktestStart] = useState(twoYearsAgoAsString());
  const [backtestEnd, setBacktestEnd] = useState(todayAsString());
  const [samplingIntervalUnit, setSamplingIntervalUnit] = useState("monthly");

  const [cash, setCash] = useState(10_000);

  const [numSymbols, setNumSymbols] = useState(10);
  const [names, setNames] = useState<string[]>([...takenNames]);
  const [err, setErr] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [assetUniverse, setAssetUniverse] = useState<string>("--");
  const [assetUniverses, setAssetUniverses] = useState<GetAssetUniversesResponse[]>([]);

  const getUniverses = async () => {
    try {
      const response = await fetch(endpoint + "/assetUniverses");
      if (response.ok) {
        const results: GetAssetUniversesResponse[] = await response.json()
        if (Object.keys(results).length === 0) {
          setErr("No universes results were retrieved");
          return;
        }
        setAssetUniverses(results);
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

  useEffect(() => {
    getUniverses()
  }, []);
  useEffect(() => {
    if (assetUniverses.length > 0) {
      setAssetUniverse(assetUniverses[0].code)
    }
  }, [assetUniverses]);

  let assetUniverseSelectOptions = assetUniverses.map(u => {
    return <option value={u.code}>{u.displayName}</option>
  })

  let found = false;
  let nextNum = 1;
  names.forEach(n => {
    if (n.includes(factorName)) {
      found = true;
      const match = n.match(/\((\d+)\)/);
      if (match) {
        const number = parseInt(match[1], 10);
        nextNum = Math.max(number + 1, nextNum)
      }
    }
  })

  const updateName = (newName: string) => {
    setFactorName(newName + "_" + samplingIntervalUnit)
  }

  useEffect(() => {
    let name = factorName;
    if (name.endsWith("_monthly")) {
      name = name.substring(0, name.indexOf("_monthly"))
    }
    if (name.endsWith("_weekly")) {
      name = name.substring(0, name.indexOf("_weekly"))
    }
    if (name.endsWith("_daily")) {
      name = name.substring(0, name.indexOf("_daily"))
    }
    updateName(name);
  }, [samplingIntervalUnit])

  const handleSubmit = async (e: any) => {
    e.preventDefault();
    setErr(null);
    setLoading(true);

    let name = factorName;
    if (found) {
      name += " (" + nextNum.toString() + ")"
    }

    const data: BacktestRequest = {
      factorOptions: {
        expression: factorExpression,
        name,
      } as FactorOptions,
      backtestStart,
      backtestEnd,
      samplingIntervalUnit,
      startCash: cash,
      numSymbols,
      userID,
      assetUniverse,
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
        jumpToAnchorOnSmallScreen("backtest-chart")
        setNames([...names, data.factorOptions.name])
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

  let rebalanceDuration = 1;
  switch (samplingIntervalUnit) {
    case "weekly": rebalanceDuration = 7; break;
    case "monthly": rebalanceDuration = 30; break;
    case "yearly": rebalanceDuration = 365; break;
  }

  const numAssetsInSelectedUniverse = assetUniverses.find(
    e => e.code === assetUniverse)?.numAssets;


  const maxDate = new Date().toISOString().split('T')[0];
  let numComputations = 0;
  if (backtestStart <= backtestEnd && backtestEnd <= maxDate) {
    if (numAssetsInSelectedUniverse) {
      // const numAssets = assetDetails.numAssets;
      numComputations = numAssetsInSelectedUniverse * daysBetweenDates(backtestStart, backtestEnd) / rebalanceDuration / 4;
    }
  }

  // TODO - attach these to the form inputs instead
  const cashInput = document.getElementById("cash");
  if (cash <= 0) {
    (cashInput as HTMLInputElement)?.setCustomValidity("Please enter more than $0.")
  } else {
    (cashInput as HTMLInputElement)?.setCustomValidity("")
  }
  const numSymbolsInput = document.getElementById("num-symbols");
  if (numSymbols <= 2) {
    (numSymbolsInput as HTMLInputElement)?.setCustomValidity("Please enter more than 2 assets.")
  } else if (numSymbols > (numAssetsInSelectedUniverse || 100)) {
    (numSymbolsInput as HTMLInputElement)?.setCustomValidity(`Please use less than ${(numAssetsInSelectedUniverse || 100)} assets.`)
  } else {
    (numSymbolsInput as HTMLInputElement)?.setCustomValidity("")
  }

  const props:FormViewProps = {
    handleSubmit,
    factorName,
    setFactorName,
    userID,
    factorExpression,
    setFactorExpression,
    updateName,
    maxDate,
    backtestStart,
    setBacktestStart,
    backtestEnd,
    setBacktestEnd,
    samplingIntervalUnit,
    setSamplingIntervalUnit,
    numSymbols,
    setNumSymbols,
    cash,
    setCash,
    assetUniverse,
    setAssetUniverse,
    assetUniverseSelectOptions,
    numComputations,
    loading,
    err
  }

  return <ClassicFormView props={props} />
}

interface FormViewProps {
  handleSubmit: (e: any) => Promise<void>,
  factorName: string,
  setFactorName: Dispatch<SetStateAction<string>>,
  userID: string,
  factorExpression: string,
  setFactorExpression: Dispatch<SetStateAction<string>>,
  updateName: (newName: string) => void,
  maxDate: string,
  backtestStart: string,
  setBacktestStart: Dispatch<SetStateAction<string>>,
  backtestEnd: string,
  setBacktestEnd: Dispatch<SetStateAction<string>>,
  samplingIntervalUnit: string,
  setSamplingIntervalUnit: Dispatch<SetStateAction<string>>,
  numSymbols: number,
  setNumSymbols: Dispatch<SetStateAction<number>>,
  cash: number,
  setCash: Dispatch<SetStateAction<number>>,
  assetUniverse: string,
  setAssetUniverse: Dispatch<SetStateAction<string>>,
  assetUniverseSelectOptions: JSX.Element[],
  numComputations: number,
  loading: boolean,
  err: string | null,
}

function ClassicFormView({
  props
}: {
  props: FormViewProps
}) {
  const {
    handleSubmit,
    factorName,
    setFactorName,
    userID,
    factorExpression,
    setFactorExpression,
    updateName,
    maxDate,
    backtestStart,
    setBacktestStart,
    backtestEnd,
    setBacktestEnd,
    samplingIntervalUnit,
    setSamplingIntervalUnit,
    numSymbols,
    setNumSymbols,
    cash,
    setCash,
    assetUniverse,
    setAssetUniverse,
    assetUniverseSelectOptions,
    numComputations,
    loading,
    err
  } = props;
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
            updateName={updateName}
          />
        </div>

        <div className='form-element'>
          <label>Backtest Range</label>
          <input
            min={'2010-01-01'}
            max={backtestEnd > maxDate ? maxDate : backtestEnd}
            required
            type="date"
            value={backtestStart}
            onChange={(e) => setBacktestStart(e.target.value)}
          />
          <p style={{ display: "inline" }}> to </p>
          <input
            max={maxDate}
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


        <div>
          <label>Number of Assets</label>
          <p className='label-subtext'>How many assets the target portfolio should hold at any time.</p>
          <input
            id="num-symbols"
            // max={numAssetsInSelectedUniverse}
            style={{ width: "80px" }}
            value={numSymbols}
            // min={3}
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
        </div>

        <div>
          <label>Starting Cash</label>
          <span style={{ fontSize: "14px" }}>$</span> <input
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
          <select value={assetUniverse} onChange={(e) => setAssetUniverse(e.target.value)}>
            {assetUniverseSelectOptions}
          </select>
        </div>

        {numComputations > 10_000 ? <p style={{ marginTop: "5px" }} className='label-subtext'>This backtest range + rebalance combination requires {numComputations.toLocaleString('en-US', { style: 'decimal' }).split('.')[0]} computations and may take up to {Math.floor(numComputations / 10000) * 10} seconds.</p> : null}

        {loading ? <img style={{ width: "40px", marginTop: "20px", marginLeft: "40px" }} src='loading.gif' /> : <button className='backtest-btn' type="submit">Run Backtest</button>}

        <Error message={err} />
      </form>
    </div>
  );
}

export function Error({ message }: { message: string | null }) {
  return message === null ? null : <>
    <div className='error-container'>
      <h4 style={{ marginBottom: "0px", marginTop: "0px" }}>That's an error.</h4>
      <p>{message}</p>
    </div>
  </>
}

function jumpToAnchorOnSmallScreen(anchorId: string) {
  // Check if the screen width is less than 600 pixels
  if (window.innerWidth < 600) {
    // Get the element with the specified anchorId
    const anchorElement = document.getElementById(anchorId);

    // Check if the element exists
    if (anchorElement) {
      // Calculate the position to scroll to
      const offset = anchorElement.getBoundingClientRect().top + window.scrollY;

      // Scroll to the element smoothly
      window.scrollTo({
        top: offset,
        behavior: 'smooth'
      });
    }
  }
}