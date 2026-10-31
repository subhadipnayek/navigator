import { Component, inject } from '@angular/core';
import { Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { CommonModule } from '@angular/common';
import { ResearchTrayService } from '../../services/research-tray.service';

@Component({
  selector: 'app-research',
  standalone: true,
  imports: [CommonModule, RouterOutlet, RouterLink, RouterLinkActive, MatButtonModule, MatIconModule],
  templateUrl: './research.html',
  styleUrl: './research.scss',
})
export class ResearchComponent {
  readonly tray = inject(ResearchTrayService);
  private readonly router = inject(Router);

  goToCompare(): void {
    this.router.navigate(['/research/compare']);
  }

  goToLab(): void {
    this.router.navigate(['/research/lab']);
  }
}
