import React, { useEffect, useState, useRef } from 'react';
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Colors,
  ChartOptions,
  ChartData,
  ChartDataset,
  Point,
  BubbleDataPoint,
} from 'chart.js';
import { Line, getDatasetAtEvent, getElementAtEvent, getElementsAtEvent } from 'react-chartjs-2';
import './app.css'

import {
  BacktestSample,
  BacktestResponse,
  BenchmarkData,
  DatasetInfo
} from './models';

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Colors
);

const App = () => {
  const chartRef = useRef<any>(null);
  const [results, updateResults] = useState<BacktestResponse[]>([]);
  const [selectedBenchmarks, updateSelectedBenchmarks] = useState(["SPY"]);

  const [benchmarkData, updateBenchmarkData] = useState<BenchmarkData[]>([]);

  let maxDate = results.reduce((maxDate, currentList) => {
    const key = Object.keys(currentList.backtestSamples).sort().slice(-1)[0];
    return key > maxDate ? key : maxDate;
  }, "");
  let minDate = results.reduce((minDate, currentList) => {
    const key = Object.keys(currentList.backtestSamples).sort()[0];
    return minDate === "" || key < minDate ? key : minDate;
  }, "");


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
              start: minDate || "2018-01-01",
              end: maxDate || "2023-01-01",
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
  const benchmarkBounds = findMinMaxDates(benchmarkData)
  if (minDate === "" && benchmarkBounds.minDate !== null) {
    minDate = benchmarkBounds.minDate
  }
  if (maxDate === "" && benchmarkBounds.maxDate !== null) {
    maxDate = benchmarkBounds.maxDate
  }

  const labels = enumerateDates(minDate, maxDate);

  const datasets: ChartDataset<"line", (number | null)[]>[] = [];

  const datasetInfo: DatasetInfo[] = [];
  benchmarkData.forEach(k => {
    datasets.push({
      label: k.symbol,
      data: labels.map(key => k.data.hasOwnProperty(key) ? k.data[key] : null),
      spanGaps: true,
    })
    datasetInfo.push({
      type: "benchmark",
      symbol: k.symbol,
    })
  })
  results.forEach(e => {
    datasets.push({
      label: e.factorName,
      data: labels.map(key => e.backtestSamples.hasOwnProperty(key) ? e.backtestSamples[key].valuePercentChange : null),
      spanGaps: true,
    })
    datasetInfo.push({
      type: "factor",
      factorName: e.factorName,
      backtestedData: Object.keys(e.backtestSamples).map(x => e.backtestSamples[x])
    })
  })

  const data: ChartData<"line", (number | Point | [number, number] | BubbleDataPoint | null)[]> = {
    labels: labels,
    datasets,
  };
  const options: ChartOptions = {
    responsive: true,
    plugins: {
      legend: {
        position: 'top',
      },
      title: {
        display: true,
        text: 'Backtested Performance',
      },
      colors: {
        forceOverride: true,
        enabled: true
      }
    },
    // onClick: (e, elements) => {
    //   console.log(elements[0].index, elements[0].datasetIndex, elements[0].element);
    // },
  };

  return <>
    <div className="centered-container">
      <div className="container">
        <div className="column" style={{ "flexGrow": 2 }}>
          <Form selectedBenchmarks={selectedBenchmarks} results={results} updateResults={updateResults} />
        </div>
        <div className="column" style={{ "flexGrow": 4 }}>
          <Line
            ref={chartRef}
            options={options}
            data={data}
            updateMode='resize'
            onClick={(event) => {
              // let x = getElementAtEvent(chartRef.current, event)[0]
              // let index = (x.index - 1) / 30
              // console.log(datasetInfo[x.datasetIndex])
              // let y = datasetInfo[x.datasetIndex].backtestedData?[index].date

            }}
          />
        </div>
      </div>
    </div>
    <BenchmarkManager selectedBenchmarks={selectedBenchmarks} updateSelectedBenchmarks={updateSelectedBenchmarks} />
  </>
}

export default App;

function Form({ results, updateResults, selectedBenchmarks }) {

  const [factorOptions, setFactorOptions] = useState({
    expression: `pricePercentChange(addDate(currentDate, 0, 0, -7),currentDate) `,
    intensity: 0.75,
    name: "test"
  });
  const [backtestStart, setBacktestStart] = useState("2020-01-02");
  const [backtestEnd, setBacktestEnd] = useState("2022-01-01");
  const [samplingIntervalUnit, setSamplingIntervalUnit] = useState("monthly");
  const [startPortfolio, setStartPortfolio] = useState(`{
      "AAPL": 10,
    "MSFT": 15,
    "GOOGL": 8
    }`);
  const [cash, setCash] = useState(0);
  const [assetSelectionMode, setAssetSelectionMode] = useState("NUM_SYMBOLS");
  const [numSymbols, setNumSymbols] = useState(10);
  const [names, setNames] = useState<string[]>([...selectedBenchmarks]);
  const [sendEnabled, setSendEnabled] = useState(true);

  const handleSubmit = async (e) => {
    e.preventDefault();
    const data = {
      factorOptions,
      backtestStart,
      backtestEnd,
      samplingIntervalUnit,
      cash,
      anchorPortfolio: JSON.parse(startPortfolio),
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
        const result = await response.json()
        const newResults = [...results, result];
        setNames([...names, factorOptions.name])
        updateResults(newResults)
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
        {assetSelectionMode === "ANCHOR_PORTFOLIO" ? <div>
          <label>Cash:</label>
          <input
            type="number"
            value={cash}
            onChange={(e) => setCash(parseFloat(e.target.value))}
          />
        </div> : null}
        <button disabled={!sendEnabled} type="submit">Submit</button>
      </form>
    </div>
  );
}

function enumerateDates(startDateStr: string, endDateStr: string) {
  const startDate = new Date(startDateStr);
  const endDate = new Date(endDateStr);

  const dates: string[] = [];
  const currentDate = new Date(startDate);

  while (currentDate <= endDate) {
    dates.push(formatDate(currentDate));
    currentDate.setDate(currentDate.getDate() + 1);
  }

  return dates;
}

function formatDate(date: Date) {
  const year = date.getFullYear();
  const month = (date.getMonth() + 1).toString().padStart(2, '0');
  const day = date.getDate().toString().padStart(2, '0');

  return `${year}-${month}-${day}`;
}

function findMinMaxDates(data: BenchmarkData[]): { minDate: string | null; maxDate: string | null } {
  let minDate: string | null = null;
  let maxDate: string | null = null;

  for (const d of data) {

    for (const date in d.data) {
      if (Object.prototype.hasOwnProperty.call(d.data, date)) {
        if (!minDate || date < minDate) {
          minDate = date;
        }
        if (!maxDate || date > maxDate) {
          maxDate = date;
        }
      }
    }
  }

  return { minDate, maxDate };
}

const BenchmarkManager = ({ selectedBenchmarks, updateSelectedBenchmarks }) => {
  const [newSymbol, setNewSymbol] = useState('');

  const handleAddBenchmark = () => {
    if (newSymbol.trim() !== '') {
      updateSelectedBenchmarks(prevBenchmarks => [...prevBenchmarks, newSymbol.trim()]);
      setNewSymbol('');
    }
  };

  const handleRemoveBenchmark = (symbolToRemove) => {
    updateSelectedBenchmarks(prevBenchmarks =>
      prevBenchmarks.filter(symbol => symbol !== symbolToRemove)
    );
  };

  return (
    <div>
      <h2>Benchmark Manager</h2>
      <div>
        <input
          type="text"
          value={newSymbol}
          onChange={event => setNewSymbol(event.target.value)}
          placeholder="Enter symbol"
        />
        <button onClick={handleAddBenchmark}>Add Benchmark</button>
      </div>
      <ul>
        {selectedBenchmarks.map(symbol => (
          <li key={symbol}>
            {symbol}{' '}
            <button onClick={() => handleRemoveBenchmark(symbol)}>Remove</button>
          </li>
        ))}
      </ul>
    </div>
  );
};



