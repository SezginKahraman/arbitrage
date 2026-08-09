import { describe, expect, it } from 'vitest';

import { formatPrice, precisionForPrice } from './format';

describe('formatPrice', () => {
  it('preserves eight decimal places for low-priced markets', () => {
    expect(formatPrice(0.01140723)).toBe('0.01140723');
  });

  it('uses a clear placeholder for invalid prices', () => {
    expect(formatPrice(Number.NaN)).toBe('—');
  });

  it.each([
    [1919.615, 2, '1,919.62'],
    [152.1234, 3, '152.123'],
    [47.61234, 4, '47.6123'],
    [1.234567, 5, '1.23457'],
  ])('formats %s with adaptive precision', (price, precision, expected) => {
    expect(precisionForPrice(price)).toBe(precision);
    expect(formatPrice(price)).toBe(expected);
  });
});
