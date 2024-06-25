import { FactorData } from "./App";
import { BacktestSnapshot } from "./models";
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
  // TODO - make this a one-liner
  const snapshotToAssetWeight = (snapshot: BacktestSnapshot): Record<string, number> => {
    let out:Record<string, number> = {}
    Object.keys(snapshot.assetMetrics).forEach(symbol => {
      out[symbol] = snapshot.assetMetrics[symbol].assetWeight
    })
    return out;
  };

  return <div className="tile fs-container">
    <div style={{ margin: "0px auto", display: "block" }}>
      <h3 style={{ marginBottom: "0px", marginTop: "0px" }}>Factor Snapshot</h3>
      <i><p className="subtext">What did "{fdDetails.name}" look like on {fdDate}?</p></i>
      <div className="container" style={{ marginTop: "30px",  width: "100%", minHeight: "0px", alignItems: "center" }}>
        <div className="column" style={{ "flexGrow": 5, maxWidth: "600px" }}>
          <AssetAllocationTable snapshot={fdData} />
        </div>
        <div className="column" style={{ "flexGrow": 2 }}>
          <div className="chart-container">
            <AssetBreakdown assetWeights={snapshotToAssetWeight(fdData)} />
            <h5 style={{ textAlign: "center" }}>Asset Allocation Breakdown</h5>

          </div>
        </div>
      </div>
    </div>

  </div>
}

const AssetAllocationTable = ({ snapshot }: { snapshot: BacktestSnapshot }) => {
  const sortedSymbols = Object.keys(snapshot.assetMetrics).sort((a, b) => snapshot.assetMetrics[b].assetWeight - snapshot.assetMetrics[a].assetWeight);
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
        {sortedSymbols.map(symbol => <tr>
          <td>{symbol}</td>
          <td>{snapshot.assetMetrics[symbol].factorScore.toFixed(2)}</td>
          <td>{(100*snapshot.assetMetrics[symbol].assetWeight).toFixed(2)}%</td>
          <td>{snapshot.assetMetrics[symbol].priceChangeTilNextResampling?.toFixed(2)}%</td>
        </tr>)}
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
