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
  BarElement,
} from 'chart.js';
import { Dispatch, SetStateAction, useState } from 'react';
import { v4 as uuid } from 'uuid';

import { Bar, Line } from 'react-chartjs-2';
import { endpoint } from '../App';


ChartJS.register(
  BarElement,
  LineElement,
);

interface BondPortfolioReturn {
  date: Date;
  totalReturn: number;
  bondReturns: { [key: string]: number };
}

interface CouponPaymentOnDate {
  bondPayments: { [key: string]: number };
  dateReceived: string;
  totalAmount: number;
}

interface BacktestBondPortfolioResult {
  couponPayments: CouponPaymentOnDate[];
  returns: BondPortfolioReturn[];
}

function BondBuilderForm(
  { updateBondBacktestData }: {
    updateBondBacktestData: Dispatch<SetStateAction<BacktestBondPortfolioResult | null>>
  }
) {
  const [backtestStart, setBacktestStart] = useState("2020-01-01");
  const [backtestEnd, setBacktestEnd] = useState("2021-01-01");
  const [startCash, setStartCash] = useState(100000);


  const submit = async () => {
    const response = await fetch(endpoint + "/backtestBondPortfolio", {
      method: "POST",
      headers: {
        "Content-Type": "application/json"
      },
      body: JSON.stringify({
        backtestStart,
        backtestEnd,
        durations: [1, 2, 3],
        startCash
      })
    })
    if (response.ok) {
      const result: BacktestBondPortfolioResult = await response.json()
      updateBondBacktestData(result);
    }
  }

  return <>
    <form>

      <label>Bond ETF Benchmark</label>
      <select>
        <option>BND</option>
        <option>SHY</option>
      </select>

      <label>Bond Ladder Durations</label>
      <select>
        <option>1M, 2M, 3M</option>
        <option>3M, 6M, 1Y</option>
        <option>1Y, 2Y, 3Y</option>
        <option>3Y, 5Y, 7Y</option>
        <option>10Y, 20Y, 30Y</option>
      </select>

      <label>Starting Cash</label>
      $<input
        value={startCash.toLocaleString()}
        onChange={(e) => {
          let x = e.target.value.replace(/,/g, '')
          if (x.length === 0) {
            x = "0";
          }
          if (!/[^0-9]/.test(x) && x.length < 12) {
            setStartCash(parseFloat(x))
          }
        }}
      />

      <div className='form-element'>
        <label>Backtest Range</label>
        <input
          min={'2018-01-01'}
          // max={backtestEnd > maxDate ? maxDate : backtestEnd}
          required
          type="date"
          value={backtestStart}
          onChange={(e) => setBacktestStart(e.target.value)}
        />
        <p style={{ display: "inline" }}> to </p>
        <input
          // max={maxDate}
          required
          type="date"
          value={backtestEnd}
          onChange={(e) => setBacktestEnd(e.target.value)}
        />
      </div>

      <br />
      <br />
      <button type="submit" onClick={e => {
        e.preventDefault();
        submit();
      }}>Run Backtest</button>
    </form>
  </>
}

export function BondBuilder() {
  const [bondBacktestData, updateBondBacktestData] = useState<BacktestBondPortfolioResult | null>(null);


  return <>
    <BondBuilderForm updateBondBacktestData={updateBondBacktestData} />
    <CouponPaymentChart couponPayments={bondBacktestData?.couponPayments} />
    <BondPortfolioPerformanceChart returns={bondBacktestData?.returns} />
  </>;
}


function CouponPaymentChart({
  couponPayments,
}: {
  couponPayments: CouponPaymentOnDate[] | undefined
}) {
  if (!couponPayments) {
    return null;
  }

  const data: ChartData<"bar", (number | Point | [number, number] | BubbleDataPoint | null)[]> = {
    labels: couponPayments.map(e => e.dateReceived),
    datasets: [{
      label: "total",
      data: couponPayments.map(e => e.totalAmount)
    }],
  };
  return <>
    <Bar data={data} />
  </>
}

function BondPortfolioPerformanceChart({
  returns
}: {
  returns: BondPortfolioReturn[] | undefined
}) {
  if (!returns) {
    return null;
  }

  const datasets: ChartDataset<"line", (number | null)[]>[] = [{
    label: "total",
    data: returns.map(e => e.totalReturn),
  }];
  const options: ChartOptions<"line"> = {}
  const data: ChartData<"line", (number | Point | [number, number] | BubbleDataPoint | null)[]> = {
    labels: returns.map(e => e.date),
    datasets,
  };

  return <>
    <Line options={options} data={data} />
  </>
}