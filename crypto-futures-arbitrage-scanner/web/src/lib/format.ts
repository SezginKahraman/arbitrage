export function precisionForPrice(price: number): number {
  const absolutePrice = Math.abs(price);

  if (absolutePrice >= 1000) return 2;
  if (absolutePrice >= 100) return 3;
  if (absolutePrice >= 10) return 4;
  if (absolutePrice >= 1) return 5;
  return 8;
}

export function formatPrice(price: number): string {
  if (!Number.isFinite(price)) return '—';

  const precision = precisionForPrice(price);
  return price.toLocaleString('en-US', {
    minimumFractionDigits: precision,
    maximumFractionDigits: precision,
  });
}
