import { describe, expect, it } from 'vitest';

import { isSymbolName } from './types';

describe('isSymbolName', () => {
  it('accepts a single-character base asset without accepting bare USDT', () => {
    expect(isSymbolName('HUSDT')).toBe(true);
    expect(isSymbolName('USDT')).toBe(false);
    expect(isSymbolName('USDTUSDT')).toBe(false);
  });
});
