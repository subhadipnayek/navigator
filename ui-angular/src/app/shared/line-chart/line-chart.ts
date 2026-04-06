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
    <div #chartWrapper class="chart-wrapper">
      <div #chartContainer class="chart-container"></div>
      <div #tooltip class="chart-tooltip"></div>
    </div>
  `,
  styles: `
    :host {
      display: block;
    }
    .chart-wrapper {
      position: relative;
    }
    .chart-container {
      width: 100%;
      height: 400px;
    }
    .chart-tooltip {
      display: none;
      position: absolute;
      top: 12px;
      z-index: 10;
      pointer-events: none;
      font-size: 0.8125rem;
      line-height: 1.5;
      background: rgba(255, 255, 255, 0.95);
      border: 1px solid #e0e0e0;
      border-radius: 6px;
      padding: 6px 10px;
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
      white-space: nowrap;
      transition:
        left 0.05s ease,
        right 0.05s ease;
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

    this.chart.subscribeCrosshairMove((param: any) => {
      this.handleCrosshairMove(param);
    });

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
    const el = this.tooltip().nativeElement;
    const container = this.chartContainer().nativeElement;

    if (!param.time || param.seriesData.size === 0) {
      el.style.display = 'none';
      return;
    }

    const timeStr =
      typeof param.time === 'string'
        ? param.time
        : `${param.time.year}-${String(param.time.month).padStart(2, '0')}-${String(param.time.day).padStart(2, '0')}`;

    const lines: string[] = [];
    lines.push(`<div style="margin-bottom:2px;font-weight:600">${timeStr}</div>`);

    for (let i = 0; i < this.seriesInstances.length; i++) {
      // First try exact match from param, then fall back to nearest lookup
      const exact = param.seriesData.get(this.seriesInstances[i]);
      const value = exact?.value ?? this.findNearestValue(this.seriesSortedData[i], timeStr);
      if (value === undefined) continue;

      const name = this.seriesNames[i];
      const color = this.seriesColors[i];
      const val = value.toLocaleString(undefined, {
        minimumFractionDigits: 2,
        maximumFractionDigits: 4,
      });
      const dot = `<span style="display:inline-block;width:8px;height:8px;border-radius:50%;background:${color};margin-right:4px;vertical-align:middle"></span>`;
      lines.push(`<div>${dot}${name ? name + ': ' : ''}${val}</div>`);
    }

    el.innerHTML = lines.join('');
    el.style.display = 'block';

    // Position tooltip near the crosshair, flip to left side if near right edge
    const tooltipWidth = el.offsetWidth;
    const containerWidth = container.clientWidth;
    const x = param.point?.x ?? 0;
    const margin = 16;

    if (x + margin + tooltipWidth > containerWidth) {
      el.style.left = '';
      el.style.right = containerWidth - x + margin + 'px';
    } else {
      el.style.right = '';
      el.style.left = x + margin + 'px';
    }
  }

  /** Binary search for the nearest data point <= the given time string (yyyy-MM-dd). */
  private findNearestValue(
    sortedData: { time: string; value: number }[],
    timeStr: string,
  ): number | undefined {
    if (sortedData.length === 0) return undefined;

    let lo = 0;
    let hi = sortedData.length - 1;

    // If before first point, return first
    if (timeStr <= sortedData[0].time) return sortedData[0].value;
    // If after last point, return last
    if (timeStr >= sortedData[hi].time) return sortedData[hi].value;

    while (lo <= hi) {
      const mid = (lo + hi) >> 1;
      if (sortedData[mid].time === timeStr) return sortedData[mid].value;
      if (sortedData[mid].time < timeStr) lo = mid + 1;
      else hi = mid - 1;
    }

    // hi is the largest index with time < timeStr, lo is the smallest with time > timeStr
    // Return whichever is closer
    if (hi < 0) return sortedData[0].value;
    if (lo >= sortedData.length) return sortedData[sortedData.length - 1].value;
    return timeStr.localeCompare(sortedData[hi].time) <= timeStr.localeCompare(sortedData[lo].time)
      ? sortedData[hi].value
      : sortedData[lo].value;
  }

  ngOnDestroy(): void {
    this.resizeObserver?.disconnect();
    this.chart?.remove();
    this.chart = null;
  }
}
