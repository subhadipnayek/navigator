/**
 * Parse "dd-MM-yyyy" to "yyyy-MM-dd" for lightweight-charts (UTCTimestamp expects yyyy-MM-dd).
 */
export function toChartDate(ddMmYyyy: string): string {
  const [d, m, y] = ddMmYyyy.split('-');
  return `${y}-${m}-${d}`;
}
