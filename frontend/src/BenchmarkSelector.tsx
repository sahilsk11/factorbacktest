import { useState } from 'react';

export default function BenchmarkManager({ selectedBenchmarks, updateSelectedBenchmarks }:any) {
  const [newSymbol, setNewSymbol] = useState('');

  const handleAddBenchmark = () => {
    if (newSymbol.trim() !== '') {
      updateSelectedBenchmarks((prevBenchmarks:any) => [...prevBenchmarks, newSymbol.trim()]);
      setNewSymbol('');
    }
  };

  const handleRemoveBenchmark = (symbolToRemove:any) => {
    updateSelectedBenchmarks((prevBenchmarks:any) =>
      prevBenchmarks.filter((symbol:string) => symbol !== symbolToRemove)
    );
  };

  return (
    <div>
      <h2>Benchmark Manager</h2>
      <div>
        <input
          type="text"
          value={newSymbol}
          onChange={event => setNewSymbol(event.target.value)}
          placeholder="Enter symbol"
        />
        <button onClick={handleAddBenchmark}>Add Benchmark</button>
      </div>
      <ul>
        {selectedBenchmarks.map((symbol:string) => (
          <li key={symbol}>
            {symbol}{' '}
            <button onClick={() => handleRemoveBenchmark(symbol)}>Remove</button>
          </li>
        ))}
      </ul>
    </div>
  );
};

