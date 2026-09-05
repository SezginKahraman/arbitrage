import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';

const themeCSS = readFileSync('src/styles/index.css', 'utf8');
const documentHTML = readFileSync('index.html', 'utf8');


describe('application theme', () => {
  it('uses a light paper-and-ink palette with accessible trading accents', () => {
    expect(themeCSS).toContain('color-scheme: light');
    expect(themeCSS).toContain('--color-terminal-ink: #f4f7f6');
    expect(themeCSS).toContain('--color-terminal-panel: #ffffff');
    expect(themeCSS).toContain('--color-terminal-text: #10211c');
    expect(themeCSS).toContain('--color-signal-mint: #087a4c');
    expect(documentHTML).toContain('<meta name="theme-color" content="#f4f7f6" />');
  });
});
