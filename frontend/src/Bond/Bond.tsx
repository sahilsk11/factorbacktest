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
import { useState } from 'react';
import { v4 as uuid } from 'uuid';

import { Bar } from 'react-chartjs-2';
import { endpoint } from '../App';


ChartJS.register(
  BarElement
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


export function BondBuilder({ }) {
  const [bondBacktestData, updateBondBacktestData] = useState<BacktestBondPortfolioResult | null>(null);
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

    <CouponPaymentChart couponPayments={bondBacktestData?.couponPayments} />
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





const d: CouponPaymentOnDate[] = [
  {
    bondPayments: {
      "27392ec3-7093-4ea0-afd9-7ed4baa14db2": 246.66666666666666,
      "821a75de-ea80-4653-937e-7e83c0181165": 251.66666666666666,
      "b597d5f7-2451-405d-ad98-177276349269": 258.3333333333333
    },
    dateReceived: "2020-02-15T00:00:00Z",
    totalAmount: 756.6666666666666
  },
  {
    bondPayments: {
      "b597d5f7-2451-405d-ad98-177276349269": 258.3333333333333,
      "c3348433-d88f-40bc-a798-3612fdbb72d6": 263.33333333333337
    },
    dateReceived: "2020-03-16T00:00:00Z",
    totalAmount: 521.6666666666667
  },
  {
    bondPayments: {
      "4274345a-2145-45c6-a45f-e1bcc4d365df": 211.66666666666666
    },
    dateReceived: "2020-04-15T00:00:00Z",
    totalAmount: 211.66666666666666
  },
  {
    bondPayments: {
      "c3348433-d88f-40bc-a798-3612fdbb72d6": 263.33333333333337
    },
    dateReceived: "2020-04-30T00:00:00Z",
    totalAmount: 263.33333333333337
  },
  {
    bondPayments: {
      "4274345a-2145-45c6-a45f-e1bcc4d365df": 211.66666666666666,
      "767903ec-cef9-492e-a694-25741b142a57": 23.33333333333334
    },
    dateReceived: "2020-05-15T00:00:00Z",
    totalAmount: 235
  },
  {
    bondPayments: {
      "767903ec-cef9-492e-a694-25741b142a57": 23.33333333333334,
      "b5bcfe3d-4d46-487a-8431-563335a032f4": 19.999999999999996
    },
    dateReceived: "2020-06-29T00:00:00Z",
    totalAmount: 43.333333333333336
  },
  {
    bondPayments: {
      "57ddae0e-5c09-4b66-9263-d19a5fb9ba1d": 26.666666666666668
    },
    dateReceived: "2020-07-14T00:00:00Z",
    totalAmount: 26.666666666666668
  },
  {
    bondPayments: {
      "767903ec-cef9-492e-a694-25741b142a57": 23.33333333333334,
      "b5bcfe3d-4d46-487a-8431-563335a032f4": 19.999999999999996
    },
    dateReceived: "2020-07-29T00:00:00Z",
    totalAmount: 43.333333333333336
  },
  {
    bondPayments: {
      "57ddae0e-5c09-4b66-9263-d19a5fb9ba1d": 26.666666666666668
    },
    dateReceived: "2020-08-28T00:00:00Z",
    totalAmount: 26.666666666666668
  },
  {
    bondPayments: {
      "00125846-dd73-4cde-9bcc-64d616068154": 18.333333333333332
    },
    dateReceived: "2020-09-12T00:00:00Z",
    totalAmount: 18.333333333333332
  },
  {
    bondPayments: {
      "00125846-dd73-4cde-9bcc-64d616068154": 18.333333333333332,
      "9366c9e1-8ea3-42dd-896f-7f3b4008cac7": 16.666666666666668
    },
    dateReceived: "2020-10-12T00:00:00Z",
    totalAmount: 35
  },
  {
    bondPayments: {
      "3fd9e0b0-b081-48ab-b1e5-3a009993b514": 16.666666666666668
    },
    dateReceived: "2020-10-27T00:00:00Z",
    totalAmount: 16.666666666666668
  },
  {
    bondPayments: {
      "9366c9e1-8ea3-42dd-896f-7f3b4008cac7": 16.666666666666668
    },
    dateReceived: "2020-11-26T00:00:00Z",
    totalAmount: 16.666666666666668
  },
  {
    bondPayments: {
      "3fd9e0b0-b081-48ab-b1e5-3a009993b514": 16.666666666666668
    },
    dateReceived: "2020-12-11T00:00:00Z",
    totalAmount: 16.666666666666668
  }
]