import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { Sidebar } from './Sidebar';

describe('Sidebar', () => {
  it('keeps desktop navigation constrained to the viewport', () => {
    render(<Sidebar onOpenSettings={vi.fn()} />);

    expect(screen.getByRole('complementary')).toHaveClass('lg:sticky', 'lg:top-0', 'lg:h-screen', 'lg:self-start');
    expect(screen.getByRole('button', { name: 'Open settings' })).toBeVisible();
  });
});
