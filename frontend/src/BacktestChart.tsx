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


import {
  enumerateDates,
  formatDate,
  findMinMaxDates,
} from "./util";

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


export default function BacktestChart({ benchmarkData, results }: any) {
  const datasets: ChartDataset<"line", (number | null)[]>[] = [];

  let maxDate = results.reduce((maxDate:any, currentList:any) => {
    const key = Object.keys(currentList.backtestSamples).sort().slice(-1)[0];
    return key > maxDate ? key : maxDate;
  }, "");
  let minDate = results.reduce((minDate:any, currentList:any) => {
    const key = Object.keys(currentList.backtestSamples).sort()[0];
    return minDate === "" || key < minDate ? key : minDate;
  }, "");
  const benchmarkBounds = findMinMaxDates(benchmarkData)


  if (minDate === "" && benchmarkBounds.minDate !== null) {
    minDate = benchmarkBounds.minDate
  }
  if (maxDate === "" && benchmarkBounds.maxDate !== null) {
    maxDate = benchmarkBounds.maxDate
  }

  const labels = enumerateDates(minDate, maxDate);

  const datasetInfo: DatasetInfo[] = [];


  benchmarkData.forEach((k:any) => {
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




  results.forEach((e:any) => {
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

  return <Line
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
}