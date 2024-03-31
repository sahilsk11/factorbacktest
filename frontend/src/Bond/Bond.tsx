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
import { Dispatch, SetStateAction, useEffect, useState } from 'react';
import "../form.css";
import "../app.css";
import "../backtest-chart.css"
import "./bond.css"


import { Bar, Line } from 'react-chartjs-2';
import { Nav, endpoint, getCookie, getOrCreateUserID } from '../App';
import { ContactModal, HelpModal } from '../Modals';
import { Error } from '../Form';


ChartJS.register(
  BarElement,
  LineElement,
);

interface BondPortfolioReturn {
  dateString: string;
  returnSinceInception: number;
  bondReturns: { [key: string]: number };
}

interface Metrics {
  stdev: number;
  averageCoupon: number;
  maxDrawdown: number;
}

interface CouponPaymentOnDate {
  bondPayments: { [key: string]: number };
  dateString: string;
  totalAmount: number;
}

interface BacktestBondPortfolioResult {
  couponPayments: CouponPaymentOnDate[];
  portfolioReturn: BondPortfolioReturn[];
  interestRates: InterestRatesOnDate[];
  metrics: Metrics;
}

interface InterestRatesOnDate {
  date: Date;
  dateString: string;
  rates: { [key: number]: number };
}

function BondBuilderForm(
  { updateBondBacktestData }: {
    updateBondBacktestData: Dispatch<SetStateAction<BacktestBondPortfolioResult | null>>
  }
) {
  const [backtestStart, setBacktestStart] = useState("2020-01-01");
  const [backtestEnd, setBacktestEnd] = useState("2024-01-01");
  const [startCash, setStartCash] = useState(1000000);
  const [selectedDuration, updateSelectedDuration] = useState(0);
  const [err, setErr] = useState<string | null>(null);

  const [loading, setLoading] = useState(false);


  useEffect(() => {
    submit();
  }, []);

  const submit = async () => {
    setLoading(true);
    setErr(null);
    try {
      const response = await fetch(endpoint + "/backtestBondPortfolio", {
        method: "POST",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify({
          backtestStart,
          backtestEnd,
          durationKey: selectedDuration,
          startCash
        })
      })
      if (response.ok) {
        const result: BacktestBondPortfolioResult = await response.json()
        updateBondBacktestData(result);
        setLoading(false);
      } else {
        setLoading(false);
        const j = await response.json()
        setErr(j.error)
      }
    } catch (error) {
      setLoading(false);
      setErr((error as Error).message);
    }
  }

  const maxDate = new Date().toISOString().split('T')[0];
  const minDate = "2000-01-01";

  return <>
    <div className='tile'>
      <h2 style={{ textAlign: "left", margin: "0px" }}>Backtest Bond Ladder</h2>
      <p className='subtext'>Pick your ladder durations and customize backtest parameters.</p>

      <form>
        <label>Bond Ladder Durations</label>
        <select value={selectedDuration} onChange={e => updateSelectedDuration(parseInt(e.target.value))}>
          <option value={0}>1M, 2M, 3M</option>
          {/* <option value={1}>3M, 6M, 1Y</option> */}
          <option value={2}>1Y, 2Y, 3Y</option>
          <option value={3}>3Y, 5Y, 7Y</option>
          <option value={4}>10Y, 20Y, 30Y</option>
        </select>

        <label>Starting Cash</label>
        <span style={{ fontSize: "14px" }}>$</span> <input
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
            min={minDate}
            max={backtestEnd > maxDate ? maxDate : backtestEnd}
            required
            type="date"
            value={backtestStart}
            onChange={(e) => setBacktestStart(e.target.value)}
          />
          <p style={{ display: "inline" }}> to </p>
          <input
            max={maxDate}
            min={minDate}
            required
            type="date"
            value={backtestEnd}
            onChange={(e) => setBacktestEnd(e.target.value)}
          />
        </div>

        <div className='form-element'>
          <label>Bond ETF Benchmark</label>
          <select>
            <option>BND</option>
            <option>SHY</option>
          </select>
        </div>

        {loading ?
          <img style={{ width: "40px", marginTop: "20px", marginLeft: "30px" }} src='loading.gif' />
          :
          <button type="submit" className='backtest-btn' onClick={e => {
            e.preventDefault();
            submit();
          }} style={{ fontSize: "13px", width: "110px", height: "35px" }}>Run Backtest</button>
        }

        <Error message={err} />
      </form>
    </div>
  </>
}

export function BondBuilder() {
  const [userID, setUserID] = useState("");
  const [bondBacktestData, updateBondBacktestData] = useState<BacktestBondPortfolioResult | null>(null);
  const [showHelpModal, setShowHelpModal] = useState(false);
  const [showContactModal, setShowContactModal] = useState(false);

  useEffect(() => {
    if (getCookie("userID") === null) {
      setShowHelpModal(true);
    }
    setUserID(getOrCreateUserID());
  }, []);

  return <>
    <Nav setShowHelpModal={setShowHelpModal} setShowContactModal={setShowContactModal} />
    <div className='centered-container' >
      <div className='container'>
        <div className="column form-wrapper">
          <BondBuilderForm updateBondBacktestData={updateBondBacktestData} />
          <ResultsOverview metrics={bondBacktestData?.metrics} />
        </div>
        <div id="backtest-chart" className="column backtest-chart-container">
          <CouponPaymentChart couponPayments={bondBacktestData?.couponPayments} />
          <BondPortfolioPerformanceChart portfolioReturns={bondBacktestData?.portfolioReturn} />
          <InterestRateChart interestRates={bondBacktestData?.interestRates} />
        </div>
      </div>
    </div>

    <ContactModal userID={""} show={showContactModal} close={() => setShowContactModal(false)} />
    <HelpModal show={showHelpModal} close={() => setShowHelpModal(false)} />
  </>;
}

function ResultsOverview({
  metrics
}: {
  metrics: Metrics | undefined
}) {
  if (!metrics) {
    return null;
  }
  return <>
    <div className='tile' style={{ marginTop: "10px" }}>
      <h4 style={{ textAlign: "left", margin: "0px" }}>Portfolio at a Glance</h4>
      <p className='subtext'>Average Coupon: {(metrics.averageCoupon*100).toFixed(2)}%</p>
      <p className='subtext'>Standard Deviation: {(metrics.stdev*100).toFixed(2)}%</p>
      <p className='subtext'>Maximum Drawdown: {(metrics.maxDrawdown*100).toFixed(2)}%</p>
      <p className='subtext'>Effective Duration: 4Y</p>
      <p className='subtext'>Yield to Worst: -4%</p>
    </div>
  </>
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
    labels: couponPayments.map(e => e.dateString),
    datasets: [{
      label: "coupon payment",
      data: couponPayments.map(e => e.totalAmount)
    }],
  };

  const options: ChartOptions<"bar"> = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        display: false
      }
    },
    scales: {
      y: {
        title: {
          display: true,
          text: 'Coupon Payment ($)', // Y-axis label
        },
      },
    }
  }

  return <>
    <div className='backtest-chart-wrapper'>
      <Bar
        data={data}
        options={options}
        updateMode='resize'
        style={{
          width: "100%",
          height: "100%"
        }} />
    </div>
  </>
}

function BondPortfolioPerformanceChart({
  portfolioReturns
}: {
  portfolioReturns: BondPortfolioReturn[] | undefined
}) {
  if (!portfolioReturns) {
    return null;
  }

  const datasets: ChartDataset<"line", (number | null)[]>[] = [{
    label: "total",
    data: portfolioReturns.map(e => 100 * e.returnSinceInception),
  }];
  const options: ChartOptions<"line"> = {
    responsive: true,
    maintainAspectRatio: false,
    scales: {
      y: {
        title: {
          display: true,
          text: 'Change since Inception (%)', // Y-axis label
        },
      },
    }
  }
  const data: ChartData<"line", (number | Point | [number, number] | BubbleDataPoint | null)[]> = {
    labels: portfolioReturns.map(e => e.dateString),
    datasets,
  };

  return <>
    <div className='backtest-chart-wrapper'>
      <Line
        options={options}
        data={data}
        updateMode='resize'

        style={{
          width: "100%",
          height: "100%"
        }}
      />
    </div>
  </>
}

function InterestRateChart({ interestRates }: {
  interestRates: InterestRatesOnDate[] | undefined
}) {

  if (!interestRates) {
    return null;
  }

  const rates: Record<number, number[]> = {
    // [duration]: rates
  };
  interestRates.forEach(entry => {
    for (const key in entry.rates) {
      const duration = parseInt(key);
      if (!rates.hasOwnProperty(duration)) {
        rates[duration] = []
      }
      const rate = 100 * entry.rates[duration];
      rates[duration].push(rate);
    }
  })

  const datasets: ChartDataset<"line", (number | null)[]>[] = Object.keys(rates).map(key => {
    const duration = parseInt(key);
    let formattedDuration = duration;
    let unit = "M"
    if (duration >= 12) {
      formattedDuration /= 12;
      unit = "Y"
    }
    return {
      label: formattedDuration.toString() + unit + " rate",
      data: rates[duration],
    }
  })
  const options: ChartOptions<"line"> = {
    responsive: true,
    maintainAspectRatio: false,
    scales: {
      y: {
        title: {
          display: true,
          text: 'Interest Rates (%)', // Y-axis label
        },
      },
    }
  }
  const data: ChartData<"line", (number | Point | [number, number] | BubbleDataPoint | null)[]> = {
    labels: interestRates.map(e => e.dateString),
    datasets,
  };

  return <>
    <div className='backtest-chart-wrapper'>
      <Line
        options={options}
        data={data}
        updateMode='resize'

        style={{
          width: "100%",
          height: "100%"
        }}
      />
    </div>
  </>
}
