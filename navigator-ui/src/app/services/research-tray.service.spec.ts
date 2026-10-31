import { TestBed } from '@angular/core/testing';
import { ResearchTrayService } from './research-tray.service';
import { Fund } from '../models/fund.model';

describe('ResearchTrayService', () => {
  let service: ResearchTrayService;

  const mockFund = (code: number, name: string): Fund => ({
    schemeCode: code,
    schemeName: name,
    isinGrowth: null,
    isinDivReinvestment: null,
    latestNav: 150.0,
    latestNavDate: '30-10-2026',
    navStartDate: '01-01-2020',
    navEndDate: '30-10-2026',
    launchDate: null,
    lastSuccessfulNavCheckUtc: null,
    navStatus: 'Active',
    isLikelyDiscontinued: false,
    fundHouse: 'UTI Mutual Fund',
    schemeType: 'Open Ended',
    schemeCategory: 'Index Fund',
  });

  beforeEach(() => {
    localStorage.clear();
    TestBed.configureTestingModule({});
    service = TestBed.inject(ResearchTrayService);
    service.clear();
  });

  afterEach(() => {
    localStorage.clear();
  });

  it('should be created and start empty', () => {
    expect(service).toBeTruthy();
    expect(service.count()).toBe(0);
    expect(service.isFull()).toBe(false);
  });

  it('should add funds up to max capacity (5)', () => {
    for (let i = 1; i <= 5; i++) {
      const added = service.addFund(mockFund(i, `Fund ${i}`));
      expect(added).toBe(true);
    }
    expect(service.count()).toBe(5);
    expect(service.isFull()).toBe(true);

    // Attempting 6th fund should fail
    const sixthAdded = service.addFund(mockFund(6, 'Fund 6'));
    expect(sixthAdded).toBe(false);
    expect(service.count()).toBe(5);
  });

  it('should not add duplicate funds', () => {
    service.addFund(mockFund(120716, 'UTI Nifty 50'));
    const dupAdded = service.addFund(mockFund(120716, 'UTI Nifty 50'));
    expect(dupAdded).toBe(false);
    expect(service.count()).toBe(1);
  });

  it('should toggle fund in and out of tray', () => {
    const fund = mockFund(120716, 'UTI Nifty 50');
    expect(service.isInTray(120716)).toBe(false);

    const toggledOn = service.toggleFund(fund);
    expect(toggledOn).toBe(true);
    expect(service.isInTray(120716)).toBe(true);

    const toggledOff = service.toggleFund(fund);
    expect(toggledOff).toBe(false);
    expect(service.isInTray(120716)).toBe(false);
  });

  it('should remove funds and clear tray', () => {
    service.addFund(mockFund(1, 'Fund 1'));
    service.addFund(mockFund(2, 'Fund 2'));
    expect(service.count()).toBe(2);

    service.removeFund(1);
    expect(service.count()).toBe(1);
    expect(service.isInTray(1)).toBe(false);
    expect(service.isInTray(2)).toBe(true);

    service.clear();
    expect(service.count()).toBe(0);
  });
});
