import { useEffect, useState } from 'react';
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

import {
  enumerateDates,
  formatDate,
  findMinMaxDates,
} from "./util";

import Form from "./Form";
import BenchmarkManager from './BenchmarkSelector';

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
            // ref={chartRef}
            options={options}
            data={data}
            // updateMode='resize'
            // onClick={(event) => {
            //   // let x = getElementAtEvent(chartRef.current, event)[0]
            //   // let index = (x.index - 1) / 30
            //   // console.log(datasetInfo[x.datasetIndex])
            //   // let y = datasetInfo[x.datasetIndex].backtestedData?[index].date

            // }}
          />
        </div>
      </div>
    </div>
    <BenchmarkManager selectedBenchmarks={selectedBenchmarks} updateSelectedBenchmarks={updateSelectedBenchmarks} />
  </>
}

export default App;





