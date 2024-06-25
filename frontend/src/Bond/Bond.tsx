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

const colors: Record<string, { borderColor: string, backgroundColor: string }> = {
  "C": {
    borderColor: '#206D69',
    backgroundColor: '#206D69',
  },
  "B": {
    borderColor: '#B38754',
    backgroundColor: '#B38754',
  },
  "A": {
    borderColor: '#1E1E1C',
    backgroundColor: '#1E1E1C',
  },
  "total": {
    borderColor: 'rgb(75, 192, 192)',
    backgroundColor: 'rgb(75, 192, 192)',
  },
}

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
  totalCoupon: number;
}

interface CouponPaymentOnDate {
  bondPayments: { [key: string]: number };
  dateString: string;
  totalAmount: number;
}

interface BondLadderOnDate {
  date: Date;
  dateStr: string;
  timeTillExpiration: number[];
  unit: string;
}

interface BacktestBondPortfolioResult {
  couponPayments: CouponPaymentOnDate[];
  portfolioReturn: BondPortfolioReturn[];
  interestRates: InterestRatesOnDate[];
  metrics: Metrics;
  bondStreams: string[][];
  bondLadder: BondLadderOnDate[];
}

interface InterestRatesOnDate {
  date: Date;
  dateString: string;
  rates: { [key: number]: number };
}

function BondBuilderForm(
  {
    updateBondBacktestData,
    userID
  }: {
    updateBondBacktestData: Dispatch<SetStateAction<BacktestBondPortfolioResult | null>>;
    userID: string;
  }
) {
  const [backtestStart, setBacktestStart] = useState("2020-01");
  const [backtestEnd, setBacktestEnd] = useState("2024-01");
  const [startCash, setStartCash] = useState(1000000);
  const [selectedDuration, updateSelectedDuration] = useState(2);
  const [err, setErr] = useState<string | null>(null);

  const [loading, setLoading] = useState(false);


  useEffect(() => {
    if (userID !== "") {
      submit();
    }
  }, [userID]);

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
          startCash,
          userID
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
    <div className='tile'  >
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
            type="month"
            value={backtestStart}
            style={{ maxWidth: "130px" }}
            onChange={(e) => setBacktestStart(e.target.value)}
          />
          <p style={{ display: "inline" }}> to </p>
          <input
            max={maxDate}
            min={minDate}
            required
            type="month"
            value={backtestEnd}
            style={{ maxWidth: "130px" }}
            onChange={(e) => setBacktestEnd(e.target.value)}
          />
        </div>

        {/* <div className='form-element'>
          <label>Bond ETF Benchmark</label>
          <select>
            <option>BND</option>
            <option>SHY</option>
          </select>
        </div> */}

        {loading ?
          <img style={{ width: "40px", marginTop: "20px", marginLeft: "30px" }} src='loading.gif' />
          :
          <button type="submit" className='backtest-btn' onClick={e => {
            e.preventDefault();
            submit();
          }} style={{ fontSize: "13px", width: "120px", height: "35px" }}>Run Backtest</button>
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
      // setShowHelpModal(true);
    }
    setUserID(getOrCreateUserID());
  }, []);

  return <>
    <Nav showLinks={false} setShowHelpModal={setShowHelpModal} setShowContactModal={setShowContactModal} />
    <div className='centered-container' >
      <div className='container'>
        <div className="column form-wrapper">
          <BondBuilderForm userID={userID} updateBondBacktestData={updateBondBacktestData} />
          <ResultsOverview metrics={bondBacktestData?.metrics} />
        </div>
        <div id="backtest-chart" className="column backtest-chart-container">
          <CouponPaymentChart
            couponPayments={bondBacktestData?.couponPayments}
            bondStreams={bondBacktestData?.bondStreams}
          />
          <BondLadderChart bondLadder={bondBacktestData?.bondLadder} />
          <BondPortfolioPerformanceChart
            portfolioReturns={bondBacktestData?.portfolioReturn}
            bondStreams={bondBacktestData?.bondStreams}
          />
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
  function numberWithCommas(x: number) {
    return Math.floor(x + 0.05).toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  }
  return <>
    <div className='tile' style={{ marginTop: "10px", marginBottom: "20px" }}>
      <h4 style={{ textAlign: "left", margin: "0px" }}>Portfolio at a Glance</h4>
      <p className='subtext'>Average Annual Coupon Rate: {(metrics.averageCoupon * 100).toFixed(2)}%</p>
      <p className='subtext'>Total Coupon Payments: ${numberWithCommas(metrics.totalCoupon)}</p>
      <p className='subtext'>Portfolio Standard Deviation: {(metrics.stdev * 100).toFixed(2)}%</p>
      <p className='subtext'>Maximum Drawdown: {(metrics.maxDrawdown * 100).toFixed(2)}%</p>
    </div>
  </>
}

function CouponPaymentChart({
  couponPayments,
  bondStreams,
}: {
  couponPayments: CouponPaymentOnDate[] | undefined
  bondStreams: string[][] | undefined
}) {
  if (!couponPayments || !bondStreams) {
    return null;
  }

  let streamData: number[][] = bondStreams.map(_ => []);
  couponPayments.forEach(paymentOnDate => {
    Object.keys(paymentOnDate.bondPayments).forEach(id => {
      const streamIndex = getBondStreamIndex(id, bondStreams)
      streamData[streamIndex].push(paymentOnDate.bondPayments[id])
    })
  })

  const datasets: ChartDataset<"bar", (number | null)[]>[] = [];

  const names = ['A', 'B', 'C']
  streamData.forEach((stream, i) => {
    datasets.push({
      label: "Bond Series " + names[i],
      data: stream,
      borderColor: colors[names[i]].borderColor,
      backgroundColor: colors[names[i]].backgroundColor,
    })
  })

  const data: ChartData<"bar", (number | Point | [number, number] | BubbleDataPoint | null)[]> = {
    labels: couponPayments.map(e => e.dateString),
    datasets,
  };

  const options: ChartOptions<"bar"> = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        display: true
      }
    },
    scales: {
      y: {
        title: {
          display: true,
          text: 'Coupon Payment ($)', // Y-axis label
        },
        stacked: true
      },
      x: {
        stacked: true
      },
    }
  }

  return <>
    <h4 className='chart-title'>Coupon Payments</h4>
    <p className='chart-description'>Every month, bonds issue payments to holders. The coupon rate is based on interest rates when the bond was purchased.</p>
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
    <hr className="chart-seperator" />
  </>
}

function getBondStreamIndex(id: string, streams: string[][]): number {
  for (let i = 0; i < streams.length; i++) {
    const stream = streams[i];
    for (let j = 0; j < stream.length; j++) {
      const idx = stream[j];
      if (id === idx) {
        return i;
      }
    }
  }
  return -1
}

function BondPortfolioPerformanceChart({
  portfolioReturns,
  bondStreams
}: {
  portfolioReturns: BondPortfolioReturn[] | undefined,
  bondStreams: string[][] | undefined
}) {
  if (!portfolioReturns || !bondStreams) {
    return null;
  }

  const datasets: ChartDataset<"line", (number | null)[]>[] = [];

  let streamData: number[][] = [];
  bondStreams.forEach(_ => {
    streamData.push([]);
  })

  portfolioReturns.forEach(dataOnDay => {
    const bondReturns = dataOnDay.bondReturns;
    Object.keys(bondReturns).forEach(id => {
      const streamIndex = getBondStreamIndex(id, bondStreams);
      streamData[streamIndex].push(100 * bondReturns[id])
    })
  });

  streamData.forEach((stream, i) => {
    const names = ['A', 'B', 'C']
    const newDataset: ChartDataset<"line", (number | null)[]> = {
      label: "Bond Series " + names[i],
      data: stream,
      borderColor: colors[names[i]].borderColor,
      backgroundColor: colors[names[i]].backgroundColor,
    }
    datasets.push(newDataset);
  })
  datasets.push({
    label: "Aggregate Portfolio",
    data: portfolioReturns.map(e => 100 * e.returnSinceInception),
    borderColor: colors["total"].borderColor,
    backgroundColor: colors["total"].backgroundColor,
  })
  const options: ChartOptions<"line"> = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {},
    },
    scales: {
      y: {
        title: {
          display: true,
          text: 'Market Value Fluctuation (%)', // Y-axis label
        },
      },
    }
  }
  const data: ChartData<"line", (number | Point | [number, number] | BubbleDataPoint | null)[]> = {
    labels: portfolioReturns.map(e => e.dateString),
    datasets,
  };

  return <>
    <h4 className='chart-title'>Market Value Fluctuation</h4>
    <p className='chart-description'>If a bond needs to be liquidated at any time, it will be sold at market price. The market price may be higher or lower than the initial purchase price, depending on current interest rates.
      <br />
      <br />
      Bonds will regress back to the purchase (par) price as they near maturity. This means bonds with longer durations are subject to more volatility than those with shorter durations.
    </p>
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
    <hr className="chart-seperator" />
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

  const names = ["A", "B", "C"]
  const datasets: ChartDataset<"line", (number | null)[]>[] = Object.keys(rates).map((key, i) => {
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
      borderColor: colors[names[i]].borderColor,
      backgroundColor: colors[names[i]].backgroundColor,
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
    <h4 className='chart-title'>Interest Rates</h4>
    <p className='chart-description'>Our simplified bond market assumes US T-bills are the only securities available. Interest rates sourced from <a style={{ color: "black" }} href="https://www.ustreasuryyieldcurve.com/" target='_blank'>here</a>.</p>
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

function BondLadderChart({ bondLadder }: {
  bondLadder: BondLadderOnDate[] | undefined
}) {

  if (!bondLadder || bondLadder.length === 0) {
    return null;
  }

  const streams: number[][] = bondLadder[0].timeTillExpiration.map(_ => []);
  const names = ['A', 'B', 'C']

  bondLadder.forEach(dataOnDate => {
    dataOnDate.timeTillExpiration.forEach((time, i) => {
      streams[i].push(time)
    })
  })

  const datasets: ChartDataset<"line", (number | null)[]>[] = streams.map((stream, i) => ({
    label: "Bond Series " + names[i],
    data: stream,
    borderColor: colors[names[i]].borderColor,
    backgroundColor: colors[names[i]].backgroundColor,
  }))
  const options: ChartOptions<"line"> = {
    responsive: true,
    maintainAspectRatio: false,
    scales: {
      y: {
        suggestedMax: 1,
        title: {
          display: true,
          text: `Remaining Bond Duration (${bondLadder[0].unit}s)`, // Y-axis label
        },
      },
    }
  }
  const data: ChartData<"line", (number | Point | [number, number] | BubbleDataPoint | null)[]> = {
    labels: bondLadder.map(e => e.dateStr),
    datasets,
  };

  return <>
    <h4 className='chart-title'>Bond Ladder</h4>
    <p className='chart-description'>Three bonds are initially purchased based on the given starting duration (such as 1Y, 2Y, 3Y). When a bond matures, a new bond is purchased to replace it at the current market interest rates. Each bond and the subsequent bonds that replace them are referenced here as a "bond series."</p>
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
    <hr className="chart-seperator" />
  </>
}
