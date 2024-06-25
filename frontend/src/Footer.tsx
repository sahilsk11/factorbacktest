import React, { useEffect, useState } from "react";
import "./app.css";
import "./footer.css";
import { endpoint } from "./App";

export default function StatsFooter({ userID }: { userID: string }) {
  const [uniqueUsers, setUniqueUsers] = useState<number | null>(null);
  const [backtests, setBacktests] = useState<number | null>(null);
  const [strategies, setStrategies] = useState<number | null>(null);

  async function getStats() {
    try {
      const response = await fetch(endpoint + "/usageStats?id=" + userID);
      const responseJson = await response.json()
      setUniqueUsers(responseJson.uniqueUsers)
      setBacktests(responseJson.backtests)
      setStrategies(responseJson.strategies)
    } catch (error) {
      console.log(error)
    }
  }

  useEffect(() => {
    if (userID !== "") {
      const timer = setTimeout(() => getStats(), 5000)
      getStats()
      return () => clearTimeout(timer);
    }
  }, [userID]);

  return <>
    <div className="footer">
      <div className="footer-text-wrapper">
        <ul>
          {/* <li className="footer-stat">Launch Date: 09/12/2023</li> */}
          <li className="footer-stat">Unique Users: {uniqueUsers}</li>
          <li className="footer-stat"># Backtests Run: {backtests}</li>
          <li className="footer-stat">Distinct Strategies Tested: {strategies}</li>
          <li className="footer-stat">Made with ❤️ in NYC</li>
        </ul>
      </div>
    </div>
  </>
}