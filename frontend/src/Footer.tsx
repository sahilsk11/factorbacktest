import React, { useEffect, useState } from "react";
import "./app.css";
import "./footer.css";
import { endpoint } from "./App";
import { GoogleAuthUser } from "./models";

export default function StatsFooter({ userID, user }: { userID: string, user: GoogleAuthUser | null }) {
  const [uniqueUsers, setUniqueUsers] = useState<number | null>(null);
  const [backtests, setBacktests] = useState<number | null>(null);
  const [strategies, setStrategies] = useState<number | null>(null);

  async function getStats() {
    try {
      const response = await fetch(endpoint + "/usageStats?id=" + userID, {
        headers: {
          "Authorization": user ? "Bearer " + user.accessToken : ""
        }
      });
      const responseJson = await response.json()
      setUniqueUsers(responseJson.uniqueUsers)
      setBacktests(responseJson.backtests)
      setStrategies(responseJson.strategies)
    } catch (error) {
      console.log(error)
    }
  }

  useEffect(() => {
    getStats();
  }, []);

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