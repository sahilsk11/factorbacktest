import { useState, useEffect, Dispatch, SetStateAction } from 'react';

import { enumerateDates } from './util';

export interface BenchmarkData {
  symbol: string;
  data: Record<string, number>;
}

export default function BenchmarkManager({
  minDate,
  maxDate,
  updateBenchmarkData
}: {
  minDate: string;
  maxDate: string;
  updateBenchmarkData: Dispatch<SetStateAction<BenchmarkData[]>>
}) {
  const [newSymbol, setNewSymbol] = useState('');
  const [selectedBenchmarks, updateSelectedBenchmarks] = useState(["SPY"]);

  useEffect(() => {
    const fetchData = async (symbol: string): Promise<BenchmarkData | null> => {
      try {
        minDate = minDate === "" ? "2018-01-01" : minDate;
        maxDate = maxDate === "" ? "2023-01-01" : maxDate;
        let granularity = "monthly";
        if (enumerateDates(minDate, maxDate).length < 60) {
          granularity = "daily"
        }
        const response = await fetch(
          'http://localhost:3009/benchmark',
          {
            method: "POST",
            headers: {
              "Content-Type": "application/json"
            },
            body: JSON.stringify({
              symbol,
              start: minDate,
              end: maxDate,
              granularity
            }),
          }
        );
        const d = await response.json();

        return {
          symbol,
          data: d,
        } as BenchmarkData;
      } catch (error) {
        console.error('Error fetching data:', error);
      }
      return null;
    };

    const fetchDataForSelectedBenchmarks = async () => {
      const promises = selectedBenchmarks.map(async (b: any) => {
        return await fetchData(b);
      });

      const newBenchmarkData = await Promise.all(promises);

      // Filter out null values (failed requests)
      const filteredBenchmarkData: BenchmarkData[] = [];
      newBenchmarkData.forEach(data => {
        if (data !== null) {
          filteredBenchmarkData.push(data);
        }
      });
      updateBenchmarkData(filteredBenchmarkData);
    }

    fetchDataForSelectedBenchmarks()
  }, [minDate, maxDate, selectedBenchmarks]);

  const handleAddBenchmark = () => {
    if (newSymbol.trim() !== '') {
      updateSelectedBenchmarks((prevBenchmarks: any) => [...prevBenchmarks, newSymbol.trim()]);
      setNewSymbol('');
    }
  };

  const handleRemoveBenchmark = (symbolToRemove: any) => {
    updateSelectedBenchmarks((prevBenchmarks: any) =>
      prevBenchmarks.filter((symbol: string) => symbol !== symbolToRemove)
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
        {selectedBenchmarks.map((symbol: string) => (
          <li key={symbol}>
            {symbol}{' '}
            <button onClick={() => handleRemoveBenchmark(symbol)}>Remove</button>
          </li>
        ))}
      </ul>
    </div>
  );
};

