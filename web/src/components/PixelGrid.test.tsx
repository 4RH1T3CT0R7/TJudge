// @vitest-environment happy-dom
import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { PixelGrid } from './PixelGrid';

describe('PixelGrid', () => {
  it('без WebGL остаётся пустым фоном, а не роняет страницу', () => {
    const { container } = render(<PixelGrid />);
    expect(container.querySelector('canvas')).toBeNull();
  });
});
