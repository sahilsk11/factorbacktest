import { FactorData } from "./App";
import { Trade } from "./Form";
import { Doughnut } from 'react-chartjs-2';
import {
  Chart as ChartJS,
  ArcElement,
  Tooltip,
  Legend
} from 'chart.js';
import "./app.css"


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
  return <div>
    <h2>{"Factor: " + fdDetails.name}</h2>
    <h4>{fdDate}</h4>
    <p>Factor Expression: {fdDetails.expression}</p>
    <p>Portfolio Value: {fdData.value.toFixed(2)} ({fdData.valuePercentChange.toFixed(2)}%)</p>
    <div className="container">
      <div className="column" style={{ "flexGrow": 1 }}>
        <Table trades={fdData.trades} />
      </div>
      <div className="column" style={{ "flexGrow": 3 }}>
        <h5>Asset Allocation Breakdown</h5>
        <AssetBreakdown assetWeights={fdData.assetWeights} />
      </div>

    </div>

  </div>
}

const Table = ({ trades }: { trades: Trade[] }) => {
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
  const keys = Object.keys(assetWeights);
  const data = {
    labels: keys,
    datasets: [
      {
        label: '% Allocation',
        data: keys.map(k => assetWeights[k] * 100),
        borderWidth: 1,
      },
    ],
  };

  return <Doughnut data={data} />;
}
