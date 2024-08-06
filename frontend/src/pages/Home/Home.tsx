import StatsFooter from "common/Footer";
import { Nav } from "common/Nav";
import { GetPublishedStrategiesResponse, GoogleAuthUser } from "models";
import { useEffect, useState } from "react";
import appStyles from "../../App.module.css";
import homeStyles from "./Home.module.css";
import { Card, Container, ListGroup, Row } from "react-bootstrap";
import { useNavigate } from "react-router-dom";
import { endpoint } from "App";
import { Bar } from "react-chartjs-2";
import {
  CategoryScale,
  Chart as ChartJS,
  Legend,
  LinearScale,
  LineElement,
  PointElement,
  TimeScale,
  Title,
  Tooltip,
  RadialLinearScale
} from 'chart.js';

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  TimeScale,
  Title,
  Tooltip,
  Legend,
  RadialLinearScale,
)

export function Home({
  user,
  setUser,
}: {
  user: GoogleAuthUser | null,
  setUser: React.Dispatch<React.SetStateAction<GoogleAuthUser | null>>;
}) {
  const [publishedStrategies, setPublishedStrategies] = useState<GetPublishedStrategiesResponse[]>([]);
  const [showHelpModal, setShowHelpModal] = useState(false);
  const [showContactModal, setShowContactModal] = useState(false);

  const navigate = useNavigate();

  async function getPublishedStrategies(): Promise<GetPublishedStrategiesResponse[]> {
    try {
      const response = await fetch(endpoint + "/publishedStrategies", {
        headers: {
          "Authorization": user ? "Bearer " + user.accessToken : ""
        }
      });
      if (!response.ok) {
        const j = await response.json()
        alert(j.error)
        console.error("Error submitting data:", response.status);
      } else {
        const j = await response.json() as GetPublishedStrategiesResponse[];
        return j
      }
    } catch (error) {
      alert((error as Error).message)
      console.error("Error:", error);
    }
    return []
  }

  useEffect(() => {
    (async () => {
      setPublishedStrategies(await getPublishedStrategies())
    })();

  }, []);

  const cards = publishedStrategies.map(ps => <StrategyCard data={ps} />)

  return <>
    <Nav loggedIn={user !== null} setUser={setUser} showLinks={false} setShowHelpModal={setShowHelpModal} setShowContactModal={setShowContactModal} />

    <div className={`${appStyles.tile} ${homeStyles.container}`}>
      <div className={homeStyles.title_container}>
        <h2 style={{ marginBottom: "0px" }}>Factor Backtest</h2>
        <p className={homeStyles.verbose_builder_subtitle}>Create and backtest factor-based investment strategies.</p>
      </div>

      <div className={homeStyles.btn_container}>
        <button
          className={`fb_btn ${homeStyles.new_btn}`}
          onClick={() => navigate("/backtest")}
        >Create Strategy +</button>
      </div>

      <Container className={homeStyles.card_container}>
        <Row style={{ display: "flex", justifyContent: "center" }}>
          {cards}
        </Row>
      </Container>
    </div>
  </>
}

function StrategyCard({
  data
}: {
  data: GetPublishedStrategiesResponse
}) {
  const weights: Record<string, number> = {};
  // stats.holdings.forEach(h => {
  //   weights[h.symbol] = h.marketValue / stats.currentValue
  // })

  const navigate = useNavigate();

  return (
    <>
      <Card
        className={`${homeStyles.card}`}
        onClick={() => navigate("/backtest?id=" + data.strategyID)}
      >
        <div style={{
          margin: "0px auto",
          display: "block",
          marginTop: "10px",
          width: "100%",
        }}
        >
          {/* <p style={{ textAlign: "center", marginTop: "65px", marginBottom: "50px" }} className={appStyles.subtext}>no data yet</p> */}
          <Bar
            data={{
              labels: ['returns', 'risk'],
              datasets: [{
                indexAxis: 'y',
                label: "strategy 1",
                data: [data.annualizedReturn, data.annualizedStandardDeviation],
                backgroundColor: [
                  '#32936F', '#95190C'
                ],
                borderWidth: 0,
                borderRadius: 10,
              }]
            }}
            options={{
              plugins: {
                legend: {
                  display: false
                }
              },
              scales: {
                x: {
                  display: false,
                  max: .50
                },
                y: {
                  max: .560
                }
              },
              maintainAspectRatio: false,
              indexAxis: "y",
              responsive: true,
            }}
            updateMode='resize'
            style={{ width: '100%', height: "100%", }}
          />
        </div>
        <Card.Body>
          <Card.Text style={{ textAlign: "center" }}>{data.strategyName}</Card.Text>

          {/* <Card.Text>
            Some quick example text to build on the card title and make up the
            bulk of the card's content.
          </Card.Text> */}
        </Card.Body>
        <ListGroup className="list-group-flush">
          <ListGroup.Item>Sharpe Ratio: {data.sharpeRatio?.toFixed(2)}</ListGroup.Item>
          <ListGroup.Item>Annualized Return: {(100 * (data.annualizedReturn || 0))?.toFixed(2)}%</ListGroup.Item>
          <ListGroup.Item>Volatility (stdev): {(100 * (data.annualizedStandardDeviation || 0)).toFixed(2)}%</ListGroup.Item>
          <ListGroup.Item>Rebalances: {data.rebalanceInterval}</ListGroup.Item>

        </ListGroup>
      </Card>
    </>
  )
}


