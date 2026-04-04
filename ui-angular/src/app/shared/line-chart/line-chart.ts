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
  template: `<div #chartContainer class="chart-container"></div>`,
  styles: `
    :host {
      display: block;
    }
    .chart-container {
      width: 100%;
      height: 400px;
    }
  `,
})
export class LineChart implements OnDestroy {
  private readonly platformId = inject(PLATFORM_ID);
  private chart: any;
  private LineSeries: any;
  private resizeObserver: ResizeObserver | null = null;
  private seriesInstances: any[] = [];

  readonly chartContainer = viewChild.required<ElementRef<HTMLDivElement>>('chartContainer');
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

    // Add new series
    for (let i = 0; i < seriesList.length; i++) {
      const series = this.chart.addSeries(this.LineSeries, {
        color: CHART_COLORS[i % CHART_COLORS.length],
        lineWidth: 2,
        priceFormat: this.priceFormat(),
        title: seriesList.length > 1 ? seriesList[i].name : '',
      });

      const sorted = [...seriesList[i].data].sort((a, b) => a.time.localeCompare(b.time));
      series.setData(sorted);
      this.seriesInstances.push(series);
    }

    this.chart.timeScale().fitContent();
  }

  ngOnDestroy(): void {
    this.resizeObserver?.disconnect();
    this.chart?.remove();
    this.chart = null;
  }
}
