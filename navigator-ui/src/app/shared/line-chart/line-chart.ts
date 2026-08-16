import {
  Component,
  ElementRef,
  input,
  effect,
  viewChild,
  OnDestroy,
  afterNextRender,
  PLATFORM_ID,
  inject,
} from '@angular/core';
import { isPlatformBrowser } from '@angular/common';
import { CHART_COLORS } from '../../shared/chart-colors';

export interface ChartSeriesData {
  name: string;
  data: { time: string; value: number }[]; // time = "yyyy-MM-dd"
}

@Component({
  selector: 'app-line-chart',
  template: `
    <div class="chart-shell">
      <div #chartContainer class="chart-host"></div>
      <div #tooltip class="chart-tooltip"></div>
    </div>
  `,
  styles: `
    :host {
      display: block;
    }

    .chart-shell {
      position: relative;
    }

    .chart-host {
      width: 100%;
      min-height: 400px;
    }

    .chart-tooltip {
      display: none;
      position: absolute;
      top: 12px;
      z-index: 3;
      max-width: min(360px, calc(100% - 24px));
      padding: 10px 12px;
      border: 1px solid rgba(15, 23, 42, 0.12);
      border-radius: 12px;
      background: rgba(255, 255, 255, 0.96);
      box-shadow: 0 14px 36px rgba(15, 23, 42, 0.16);
      color: #0f172a;
      font-size: 12px;
      line-height: 1.45;
      pointer-events: none;
      white-space: normal;
    }
  `,
})
export class LineChart implements OnDestroy {
  private readonly platformId = inject(PLATFORM_ID);
  private chart: any;
  private LineSeries: any;
  private resizeObserver: ResizeObserver | null = null;
  private seriesInstances: any[] = [];
  private seriesNames: string[] = [];
  private seriesColors: string[] = [];
  private seriesSortedData: { time: string; value: number }[][] = [];

  readonly chartContainer = viewChild.required<ElementRef<HTMLDivElement>>('chartContainer');
  readonly tooltip = viewChild.required<ElementRef<HTMLDivElement>>('tooltip');
  readonly seriesList = input.required<ChartSeriesData[]>();
  readonly yAxisLabel = input<string>('');
  readonly priceFormat = input<{ type: string; precision: number; minMove: number }>({
    type: 'price',
    precision: 2,
    minMove: 0.01,
  });

  constructor() {
    afterNextRender(() => {
      this.initChart();
    });

    effect(() => {
      const data = this.seriesList();
      if (this.chart && data.length > 0) {
        this.updateSeries(data);
      }
    });
  }

  private async initChart(): Promise<void> {
    if (!isPlatformBrowser(this.platformId)) return;

    const { createChart, LineSeries } = await import('lightweight-charts');
    this.LineSeries = LineSeries;
    const container = this.chartContainer().nativeElement;

    this.chart = createChart(container, {
      width: container.clientWidth || 800,
      height: 400,
      layout: {
        background: { color: '#ffffff' },
        textColor: '#333',
        fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
      },
      grid: {
        vertLines: { color: '#f0f0f0' },
        horzLines: { color: '#f0f0f0' },
      },
      rightPriceScale: {
        borderColor: '#e0e0e0',
      },
      timeScale: {
        borderColor: '#e0e0e0',
        timeVisible: false,
      },
      crosshair: {
        mode: 0,
      },
    });

    this.resizeObserver = new ResizeObserver(() => {
      if (this.chart) {
        this.chart.applyOptions({ width: container.clientWidth });
      }
    });
    this.resizeObserver.observe(container);

    this.chart.subscribeCrosshairMove((param: any) => this.handleCrosshairMove(param));

    const data = this.seriesList();
    if (data.length > 0) {
      this.updateSeries(data);
    }
  }

  private updateSeries(seriesList: ChartSeriesData[]): void {
    if (!this.chart) return;

    // Remove old series
    for (const s of this.seriesInstances) {
      this.chart.removeSeries(s);
    }
    this.seriesInstances = [];
    this.seriesNames = [];
    this.seriesColors = [];
    this.seriesSortedData = [];

    // Add new series
    for (let i = 0; i < seriesList.length; i++) {
      const color = CHART_COLORS[i % CHART_COLORS.length];
      const series = this.chart.addSeries(this.LineSeries, {
        color,
        lineWidth: 2,
        priceFormat: this.priceFormat(),
        lastValueVisible: false,
        title: '',
      });

      const sorted = [...seriesList[i].data].sort((a, b) => a.time.localeCompare(b.time));
      series.setData(sorted);
      this.seriesInstances.push(series);
      this.seriesNames.push(seriesList[i].name);
      this.seriesColors.push(color);
      this.seriesSortedData.push(sorted);
    }

    this.chart.timeScale().fitContent();
  }

  private handleCrosshairMove(param: any): void {
    const tooltip = this.tooltip().nativeElement;
    const container = this.chartContainer().nativeElement;

    if (!param.point || !param.time || param.point.x < 0 || param.point.y < 0) {
      tooltip.style.display = 'none';
      return;
    }

    const timeStr = this.formatCrosshairTime(param.time);
    const rows: string[] = [`<strong>${timeStr}</strong>`];

    for (let i = 0; i < this.seriesInstances.length; i++) {
      const exact = param.seriesData.get(this.seriesInstances[i]);
      const value = exact?.value ?? this.findNearestValue(this.seriesSortedData[i], timeStr);
      if (value === undefined) continue;

      rows.push(
        `<div style="display:flex;gap:8px;align-items:center;margin-top:5px">
          <span style="width:9px;height:9px;border-radius:999px;background:${this.seriesColors[i]};flex:0 0 auto"></span>
          <span style="overflow:hidden;text-overflow:ellipsis">${this.escapeHtml(this.seriesNames[i])}</span>
          <strong style="margin-left:auto">${this.formatValue(value)}</strong>
        </div>`,
      );
    }

    if (rows.length === 1) {
      tooltip.style.display = 'none';
      return;
    }

    tooltip.innerHTML = rows.join('');
    tooltip.style.display = 'block';

    const margin = 16;
    const tooltipWidth = tooltip.offsetWidth;
    const x = param.point.x;
    const left =
      x + margin + tooltipWidth > container.clientWidth
        ? Math.max(12, x - tooltipWidth - margin)
        : x + margin;

    tooltip.style.left = `${left}px`;
  }

  private formatCrosshairTime(time: unknown): string {
    if (typeof time === 'string') return time;
    if (typeof time === 'object' && time && 'year' in time && 'month' in time && 'day' in time) {
      const t = time as { year: number; month: number; day: number };
      return `${t.year}-${String(t.month).padStart(2, '0')}-${String(t.day).padStart(2, '0')}`;
    }
    return String(time);
  }

  private formatValue(value: number): string {
    return value.toLocaleString(undefined, {
      minimumFractionDigits: this.priceFormat().precision > 0 ? 2 : 0,
      maximumFractionDigits: this.priceFormat().precision,
    });
  }

  private findNearestValue(
    sortedData: { time: string; value: number }[],
    timeStr: string,
  ): number | undefined {
    if (sortedData.length === 0) return undefined;
    let lo = 0;
    let hi = sortedData.length - 1;
    while (lo <= hi) {
      const mid = (lo + hi) >> 1;
      if (sortedData[mid].time === timeStr) return sortedData[mid].value;
      if (sortedData[mid].time < timeStr) lo = mid + 1;
      else hi = mid - 1;
    }
    if (hi < 0) return sortedData[0].value;
    if (lo >= sortedData.length) return sortedData[sortedData.length - 1].value;
    return sortedData[hi].value;
  }

  private escapeHtml(value: string): string {
    return value
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  ngOnDestroy(): void {
    this.resizeObserver?.disconnect();
    this.chart?.remove();
    this.chart = null;
  }
}
