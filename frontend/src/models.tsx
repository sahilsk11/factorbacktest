export interface BacktestSample {
  valuePercentChange: number;
  value: number;
  date: string;
}

export interface BacktestResponse {
  factorName: string;
  backtestSamples: Record<string, BacktestSample>;
}

export interface BenchmarkData {
  symbol: string;
  data: Record<string, number>;
}

export interface DatasetInfo {
  type: string;
  symbol?: string | undefined;
  factorName?: string | undefined;
  factorExpression?: string | undefined;
  backtestedData?: BacktestSample[];
}