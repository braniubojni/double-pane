import { afterEach, describe, expect, it, vi } from 'vitest';
import { triggerCalcSizes } from './helpers';

afterEach(() => {
  document.body.replaceChildren();
});

describe('triggerCalcSizes', () => {
  it('clicks the folder-sizes button for the given pane', () => {
    const left = document.createElement('button');
    left.dataset.testid = 'btn-folder-sizes-left';
    const onClick = vi.fn();
    left.addEventListener('click', onClick);
    const right = document.createElement('button');
    right.dataset.testid = 'btn-folder-sizes-right';
    const onRight = vi.fn();
    right.addEventListener('click', onRight);
    document.body.append(left, right);

    triggerCalcSizes('left');

    expect(onClick).toHaveBeenCalledOnce();
    expect(onRight).not.toHaveBeenCalled();
  });

  it('clicks the right pane button without touching the left one', () => {
    const right = document.createElement('button');
    right.dataset.testid = 'btn-folder-sizes-right';
    const onClick = vi.fn();
    right.addEventListener('click', onClick);
    document.body.append(right);

    triggerCalcSizes('right');

    expect(onClick).toHaveBeenCalledOnce();
  });

  it('does nothing when the button is missing', () => {
    expect(() => triggerCalcSizes('left')).not.toThrow();
  });

  it('does nothing when the match is not an HTMLElement', () => {
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('data-testid', 'btn-folder-sizes-left');
    const onClick = vi.fn();
    svg.addEventListener('click', onClick);
    document.body.append(svg);

    triggerCalcSizes('left');

    expect(onClick).not.toHaveBeenCalled();
  });
});
