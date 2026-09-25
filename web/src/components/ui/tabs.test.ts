// @vitest-environment happy-dom
import type { KeyboardEvent } from 'react';
import { expect, it } from 'vitest';
import { handleTabListKeyDown } from './tabs';

it('стрелки и Home/End переключают вкладки по кругу', () => {
  const list = document.createElement('div');
  const clicked: string[] = [];
  for (const id of ['a', 'b', 'c']) {
    const tab = document.createElement('button');
    tab.id = id;
    tab.setAttribute('role', 'tab');
    tab.addEventListener('click', () => clicked.push(id));
    list.appendChild(tab);
  }
  document.body.appendChild(list);

  const press = (key: string) =>
    handleTabListKeyDown({ key, currentTarget: list, preventDefault: () => {} } as unknown as KeyboardEvent<HTMLElement>);

  document.getElementById('a')!.focus();
  press('ArrowLeft');
  press('ArrowRight');
  press('Home');
  press('End');
  press('Enter');

  expect(clicked).toEqual(['c', 'a', 'a', 'c']);
  expect(document.activeElement?.id).toBe('c');
});
