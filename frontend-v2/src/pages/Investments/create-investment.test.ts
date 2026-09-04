import { describe, expect, it } from 'vitest';

import {
  DEFAULT_CREATE_INVESTMENT_FORM,
  validateCreateInvestmentForm,
  type CreateInvestmentForm,
} from './create-investment';

describe('validateCreateInvestmentForm', () => {
  const validForm: CreateInvestmentForm = {
    ...DEFAULT_CREATE_INVESTMENT_FORM,
  };

  it('accepts a valid form', () => {
    expect(validateCreateInvestmentForm(validForm)).toBeNull();
  });

  it('requires a strategy name', () => {
    expect(validateCreateInvestmentForm({ ...validForm, strategyName: '   ' })).toMatch(
      /strategy name/i,
    );
  });

  it('requires amount greater than zero', () => {
    expect(validateCreateInvestmentForm({ ...validForm, amountDollars: 0 })).toMatch(
      /greater than/i,
    );
    expect(validateCreateInvestmentForm({ ...validForm, amountDollars: -100 })).toMatch(
      /greater than/i,
    );
  });

  it('requires a factor expression', () => {
    expect(validateCreateInvestmentForm({ ...validForm, factorExpression: '  ' })).toMatch(
      /factor expression/i,
    );
  });

  it('requires an asset universe', () => {
    expect(validateCreateInvestmentForm({ ...validForm, assetUniverse: '' })).toMatch(
      /asset universe/i,
    );
  });

  it('requires at least three assets', () => {
    expect(validateCreateInvestmentForm({ ...validForm, numAssets: 2 })).toMatch(/3 assets/i);
  });
});
