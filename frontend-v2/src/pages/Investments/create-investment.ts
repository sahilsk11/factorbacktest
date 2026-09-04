import { apiClient } from '@/lib/api';
import type { RebalanceInterval } from '@/pages/Builder/types';

export interface CreateInvestmentForm {
  strategyName: string;
  amountDollars: number;
  factorExpression: string;
  assetUniverse: string;
  rebalanceInterval: RebalanceInterval;
  numAssets: number;
}

export interface BookmarkStrategyResponse {
  message: string;
  savedStrategyID: string;
}

export interface InvestInStrategyResponse {
  success: boolean;
}

export const DEFAULT_CREATE_INVESTMENT_FORM: CreateInvestmentForm = {
  strategyName: 'exploding rockets',
  amountDollars: 2000,
  factorExpression: 'pricePercentChange(addDate(currentDate, 0, -6, 0), currentDate)',
  assetUniverse: 'SPY_TOP_300',
  rebalanceInterval: 'daily',
  numAssets: 3,
};

export function validateCreateInvestmentForm(form: CreateInvestmentForm): string | null {
  if (!form.strategyName.trim()) {
    return 'Strategy name is required';
  }
  if (!Number.isFinite(form.amountDollars) || form.amountDollars <= 0) {
    return 'Investment amount must be greater than $0';
  }
  if (!form.factorExpression.trim()) {
    return 'Factor expression is required';
  }
  if (!form.assetUniverse.trim()) {
    return 'Asset universe is required';
  }
  if (!form.rebalanceInterval) {
    return 'Rebalance interval is required';
  }
  if (!Number.isInteger(form.numAssets) || form.numAssets < 3) {
    return 'Hold at least 3 assets';
  }
  return null;
}

export async function createInvestment(form: CreateInvestmentForm): Promise<void> {
  const validationError = validateCreateInvestmentForm(form);
  if (validationError) {
    throw new Error(validationError);
  }

  const bookmark = await apiClient.post<BookmarkStrategyResponse>('/bookmarkStrategy', {
    expression: form.factorExpression.trim(),
    name: form.strategyName.trim(),
    rebalanceInterval: form.rebalanceInterval,
    numAssets: form.numAssets,
    assetUniverse: form.assetUniverse,
    bookmark: true,
  });

  await apiClient.post<InvestInStrategyResponse>('/investInStrategy', {
    strategyID: bookmark.savedStrategyID,
    amountDollars: Math.round(form.amountDollars),
  });
}
