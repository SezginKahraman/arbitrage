import { describe, expect, it } from 'vitest';

import { pageFromPath, pathForPage } from './navigation';

describe('Futures navigation', () => {
  it('maps the Futures workspace without changing existing routes', () => {
    expect(pageFromPath('/futures')).toBe('futures');
    expect(pageFromPath('/futures/COTI_USDT')).toBe('futures');
    expect(pathForPage('futures')).toBe('/futures');
    expect(pageFromPath('/opportunities')).toBe('opportunities');
    expect(pageFromPath('/alerts')).toBe('alerts');
    expect(pageFromPath('/')).toBe('scanner');
  });
});
