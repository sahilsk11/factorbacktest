import { Dispatch } from 'react';

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

import { Line } from 'react-chartjs-2';


import {
  enumerateDates,
  minMaxDates,
} from "./util";


import { FactorData } from './App';
import { BenchmarkData } from './BenchmarkSelector';

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


export default function BacktestChart({
  benchmarkData,
  factorData,
  updateInspectFactorDataIndex,
  updateInspectFactorDataDate
}: {
  benchmarkData: BenchmarkData[];
  factorData: FactorData[];
  updateInspectFactorDataIndex: Dispatch<React.SetStateAction<number | null>>;
  updateInspectFactorDataDate: Dispatch<React.SetStateAction<string | null>>;
}) {
  const datasets: ChartDataset<"line", (number | null)[]>[] = [];

  let { min: minDate, max: maxDate } = minMaxDates(factorData);

  minDate = minDate === "" ? "2020-01-01" : minDate;
  maxDate = maxDate === "" ? "2022-01-01" : maxDate;

  const labels = enumerateDates(minDate, maxDate);

  benchmarkData.forEach((k: BenchmarkData) => {
    datasets.push({
      label: k.symbol,
      data: labels.map(key => k.data.hasOwnProperty(key) ? k.data[key] : null),
      spanGaps: true,
    })
  })

  factorData.forEach((e: FactorData) => {
    datasets.push({
      label: e.name,
      data: labels.map(key => e.data.hasOwnProperty(key) ? e.data[key].valuePercentChange : null),
      spanGaps: true,
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
    onClick: (_, elements) => {
      elements.forEach(e => {
        if (e.datasetIndex >= benchmarkData.length) {
          const factorIndex = e.datasetIndex - benchmarkData.length;
          const date = labels[e.index];
          updateInspectFactorDataIndex(factorIndex);
          updateInspectFactorDataDate(date);
        }
      })
    },
  };

  return <Line
    options={options}
    data={data}
    updateMode='resize'
  />
}