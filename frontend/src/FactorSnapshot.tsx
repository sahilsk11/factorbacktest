import { FactorData } from "./App";
import { Trade } from "./Form";
import { Doughnut } from 'react-chartjs-2';
import {
  Chart as ChartJS,
  ArcElement,
  Tooltip,
  Legend,
  ChartOptions
} from 'chart.js';
import "./app.css";
import "./factor-snapshot.css";


export default function InspectFactorData({
  fdIndex,
  fdDate,
  factorData
}: {
  fdIndex: number | null;
  fdDate: string | null;
  factorData: FactorData[];
}) {
  if (fdIndex === null || fdDate === null || factorData.length === 0) {
    return null;
  }
  const fdDetails = factorData[fdIndex];
  const fdData = fdDetails.data[fdDate];
  return <div className="tile fs-container">
    <div style={{ margin: "0px auto", display: "block" }}>
      <h3 style={{ marginBottom: "0px", marginTop: "0px" }}>Factor Snapshot</h3>
      <i><p className="subtext">What did "{fdDetails.name}" look like on {fdDate}?</p></i>
      <div className="container" style={{ marginTop: "30px", width: "100%", minHeight: "none" }}>
        <div className="column" style={{ "flexGrow": 3, maxWidth: "600px" }}>
          <AssetAllocationTable trades={fdData.trades} />
        </div>
        <div className="column" style={{ "flexGrow": 1 }}>
          <div className="chart-container">
            <AssetBreakdown assetWeights={fdData.assetWeights} />
            <h5 style={{ textAlign: "center" }}>Asset Allocation Breakdown</h5>

          </div>
        </div>
      </div>
    </div>

  </div>
}

const AssetAllocationTable = ({ trades }: { trades: Trade[] }) => {
  return (
    <table className="table">
      <thead>
        <tr>
          <th>Symbol</th>
          <th>Factor Score</th>
          <th>Portfolio Allocation</th>
          <th>Price Change til Next Resampling</th>

        </tr>
      </thead>
      <tbody>
        <tr>
          <td>AAPL</td>
          <td>45.2</td>
          <td>40%</td>
          <td>10%</td>
        </tr>
        <tr>
          <td>AAPL</td>
          <td>45.2</td>
          <td>40%</td>
          <td>10%</td>
        </tr>
        <tr>
          <td>AAPL</td>
          <td>45.2</td>
          <td>40%</td>
          <td>10%</td>
        </tr>
        <tr>
          <td>AAPL</td>
          <td>45.2</td>
          <td>40%</td>
          <td>10%</td>
        </tr>
      </tbody>
    </table>
  );
};

const TradesTable = ({ trades }: { trades: Trade[] }) => {
  return (
    <table className="table">
      <thead>
        <tr>
          <th>Action</th>
          <th>Quantity</th>
          <th>Symbol</th>
          <th>Price</th>
        </tr>
      </thead>
      <tbody>
        {trades.map((trade, index) => (
          <tr key={index} className={trade.action === 'BUY' ? 'buy' : 'sell'}>
            <td>{trade.action}</td>
            <td>{trade.quantity}</td>
            <td>{trade.symbol}</td>
            <td>{trade.price}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
};

ChartJS.register(ArcElement, Tooltip, Legend);

const AssetBreakdown = ({
  assetWeights
}: {
  assetWeights: Record<string, number>
}) => {
  const assetData = Object.keys(assetWeights).map((key) => ({
    asset: key,
    allocation: assetWeights[key] * 100,
  }));

  // Sort the assetData array by allocation (percentage) in descending order
  assetData.sort((a, b) => b.allocation - a.allocation);

  // Extract the sorted keys and data values
  const labels = assetData.map((item) => item.asset);
  const dataValues = assetData.map((item) => item.allocation);

  const options: ChartOptions<"doughnut"> = {
    plugins: {
      legend: {
        display: false,
        position: "right"
      },
    },
  };


  const data = {
    labels,
    datasets: [
      {
        label: '% Allocation',
        data: dataValues,
        borderWidth: 1,
      },
    ],
  };

  return <Doughnut data={data} options={options} />;
}
