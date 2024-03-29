
export function BondBuilder({ }) {
  return <>
    <form>
      <label>Bond One Duration</label>
      <select>
        <option>1 month</option>
        <option>3 month</option>
        <option>4 month</option>
        <option>6 month</option>
      </select>

      <label>Bond Two Duration</label>
      <select>
        <option>1 month</option>
        <option>3 month</option>
        <option>4 month</option>
        <option>6 month</option>
      </select>

      <label>Bond Three Duration</label>
      <select>
        <option>1 month</option>
        <option>3 month</option>
        <option>4 month</option>
        <option>6 month</option>
      </select>

      <label>Bond ETF Benchmark</label>
      <select>
        <option>BND</option>
        <option>SHY</option>
      </select>

      <label>Starting Cash</label>
      <input value="100000" />

      <div className='form-element'>
          <label>Backtest Range</label>
          <input
            min={'2018-01-01'}
            // max={backtestEnd > maxDate ? maxDate : backtestEnd}
            required
            type="date"
            // value={backtestStart}
            // onChange={(e) => setBacktestStart(e.target.value)}
          />
          <p style={{ display: "inline" }}> to </p>
          <input
            // max={maxDate}
            required
            type="date"
            // value={backtestEnd}
            // onChange={(e) => setBacktestEnd(e.target.value)}
          />
        </div>



      <br />
      <br />
      <button type="submit">Run Backtest</button>
    </form>
  </>;
}
