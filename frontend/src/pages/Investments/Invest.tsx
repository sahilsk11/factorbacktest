import { GoogleAuthUser, GetSavedStrategiesResponse, InvestInStrategyRequest, GetInvestmentsResponse } from "../../models";
import { Nav } from "common/Nav"
import { Dispatch, SetStateAction, useEffect, useState } from "react";
import { ContactModal, HelpModal } from "common/Modals";
import { endpoint } from "App";
import { formatDate } from "../../util";
import { useNavigate, useSearchParams } from "react-router-dom";

import { AssetBreakdown } from "../Backtest/FactorSnapshot";
import { useAuth } from "auth";
import LoginModal from "common/AuthModals";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { LineChart } from "@/components/ui/line-chart"

export default function Invest({
  user,
  setUser,
}: {
  user: GoogleAuthUser | null,
  setUser: React.Dispatch<React.SetStateAction<GoogleAuthUser | null>>;
}) {
  const [showHelpModal, setShowHelpModal] = useState(false);
  const [showContactModal, setShowContactModal] = useState(false);
  const [activeInvestments, setActiveInvestments] = useState<GetInvestmentsResponse[]>([]);
  const [showLoginModal, setShowLoginModal] = useState(false);

  const { loading, session } = useAuth();

  useEffect(() => {
    if (session) {
      getInvestments()
    } else if (!loading) {
      setActiveInvestments([]);
      setShowLoginModal(true);
    }
  }, [loading, session])

  const navigate = useNavigate();

  async function getInvestments() {
    try {
      const response = await fetch(endpoint + "/activeInvestments", {
        headers: {
          "Authorization": session ? "Bearer " + session.access_token : ""
        }
      });
      if (!response.ok) {
        const j = await response.json()
        alert(j.error)
        console.error("Error submitting data:", response.status);
      } else {
        const j = await response.json()
        setActiveInvestments(j)
      }
    } catch (error) {
      alert((error as Error).message)
      console.error("Error:", error);
    }
  }

  return (
    <>
      <Nav loggedIn={user !== null} setUser={setUser} showLinks={false} />
      {showLoginModal && <LoginModal show={showLoginModal} close={() => {
        setShowLoginModal(false);
        if (!session) navigate("/");
      }} />}
      <div className="container mx-auto px-4 py-8">
        <h2 className="text-3xl font-bold mb-2">Active Investments</h2>
        <p className="text-gray-600 mb-6">View your active investment strategies.</p>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
          {activeInvestments.map((investment) => (
            <InvestmentTile key={investment.investmentID} stats={investment} />
          ))}
        </div>
      </div>
    </>
  )
}

function InvestmentTile({ stats }: { stats: GetInvestmentsResponse }) {
  const latestTrade = stats.completedTrades.length > 0
    ? formatDate(new Date(stats.completedTrades.reduce((latest, trade) => 
        trade.filledAt > latest.filledAt ? trade : latest
      ).filledAt))
    : "n/a";

  const percentReturn = (stats.percentReturnFraction * 100).toFixed(2);
  const isPositive = parseFloat(percentReturn) >= 0;

  // Generate mock data for the chart
  const chartData = Array.from({ length: 30 }, (_, i) => ({
    date: new Date(Date.now() - (29 - i) * 24 * 60 * 60 * 1000).toISOString().slice(0, 10),
    value: stats.currentValue * (1 + Math.random() * 0.1 - 0.05)
  }));

  return (
    <Card className="w-full max-w-sm">
      <CardHeader className="pb-2">
        <CardTitle className="text-lg font-semibold">{stats.strategy.strategyName}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <div className="text-2xl font-bold">${stats.currentValue.toFixed(2)}</div>
            <div className={`px-2 py-1 rounded-full text-sm font-medium ${
              isPositive ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'
            }`}>
              {percentReturn}%
            </div>
          </div>
          <div className="h-32">
            <LineChart
              data={chartData}
              categories={["value"]}
              index="date"
              colors={["blue"]}
              yAxisWidth={40}
              showXAxis={false}
              showYAxis={false}
              showLegend={false}
              showGridLines={false}
              curveType="monotone"
            />
          </div>
          <div className="grid grid-cols-2 gap-2 text-sm">
            <div>
              <span className="text-gray-500">Inception:</span>
              <br />
              {stats.startDate}
            </div>
            <div>
              <span className="text-gray-500">Last Trade:</span>
              <br />
              {latestTrade}
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}