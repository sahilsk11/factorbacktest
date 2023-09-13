import React from "react";
import "./app.css";
import "./footer.css";

export default function StatsFooter() {
  return <>
  <div className="footer">
    <div className="footer-text-wrapper">
      <ul>
        {/* <li className="footer-stat">Launch Date: 09/12/2023</li> */}
        <li className="footer-stat">Unique Users: 19</li>
        <li className="footer-stat"># Backtests Run: 140</li>
        <li className="footer-stat">Distinct Strategies Tested: 10</li>
        <li className="footer-stat">Made with ❤️ in NYC</li>
      </ul>
    </div>
  </div>
  </>
}