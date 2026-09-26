// @vitest-environment happy-dom
import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes, Link } from 'react-router-dom';
import { AnimatedOutlet } from './AnimatedOutlet';

describe('AnimatedOutlet', () => {
  it('после перехода фокус на h1 новой страницы, при первом открытии - нет', async () => {
    render(
      <MemoryRouter initialEntries={['/a']}>
        <main id="main-content">
          <Routes>
            <Route path="/" element={<AnimatedOutlet />}>
              <Route path="a" element={<><h1>Страница A</h1><Link to="/b">дальше</Link></>} />
              <Route path="b" element={<h1>Страница B</h1>} />
            </Route>
          </Routes>
        </main>
      </MemoryRouter>
    );
    expect(document.activeElement).toBe(document.body);

    fireEvent.click(screen.getByText('дальше'));
    const h1 = await screen.findByText('Страница B');
    await waitFor(() => expect(document.activeElement).toBe(h1));
  });
});
