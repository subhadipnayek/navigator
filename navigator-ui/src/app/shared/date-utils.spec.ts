import { toChartDate } from './date-utils';

describe('toChartDate', () => {
  it('converts dd-MM-yyyy to yyyy-MM-dd', () => {
    expect(toChartDate('30-10-2026')).toBe('2026-10-30');
    expect(toChartDate('01-01-2024')).toBe('2024-01-01');
  });

  it('preserves yyyy-MM-dd format', () => {
    expect(toChartDate('2026-10-30')).toBe('2026-10-30');
  });

  it('strips ISO-8601 timestamp and formats to yyyy-MM-dd', () => {
    expect(toChartDate('2026-10-30T22:45:20.000Z')).toBe('2026-10-30');
    expect(toChartDate('2026-10-30T00:00:00Z')).toBe('2026-10-30');
  });

  it('handles empty string gracefully', () => {
    expect(toChartDate('')).toBe('');
  });
});
