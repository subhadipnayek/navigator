/**
 * Robust date parser converting various backend date formats:
 * - "dd-MM-yyyy" (e.g. "30-10-2026")
 * - "yyyy-MM-dd" (e.g. "2026-10-30")
 * - ISO-8601 timestamps (e.g. "2026-10-30T22:45:20.000Z")
 * to strict "yyyy-MM-dd" expected by Lightweight Charts UTCTimestamp.
 */
export function toChartDate(dateStr: string): string {
  if (!dateStr) return '';

  // Strip ISO time component if present
  if (dateStr.includes('T')) {
    dateStr = dateStr.split('T')[0];
  }

  // Check if already in yyyy-MM-dd format
  if (/^\d{4}-\d{2}-\d{2}$/.test(dateStr)) {
    return dateStr;
  }

  // Parse hyphen-delimited format (dd-MM-yyyy)
  const parts = dateStr.split('-');
  if (parts.length === 3) {
    if (parts[0].length === 4) {
      // yyyy-MM-dd
      return dateStr;
    }
    const [d, m, y] = parts;
    return `${y}-${m.padStart(2, '0')}-${d.padStart(2, '0')}`;
  }

  return dateStr;
}
