import { afterEach, describe, expect, it, vi } from 'vitest';
import { dropModeForDrag, subscribeDropMode } from './dragState';

const setUserAgent = (ua: string) => {
  Object.defineProperty(navigator, 'userAgent', { value: ua, configurable: true });
};

const key = (type: string, init: KeyboardEventInit) => {
  window.dispatchEvent(new KeyboardEvent(type, { bubbles: true, ...init }));
};

const mouse = (type: string, init: MouseEventInit) => {
  window.dispatchEvent(new MouseEvent(type, { bubbles: true, ...init }));
};

afterEach(() => {
  window.dispatchEvent(new Event('blur'));
  setUserAgent('Mozilla/5.0');
});

describe('drop modifier listeners', () => {
  it('starts as copy and notifies subscribers once per change', () => {
    setUserAgent('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)');
    const heard = vi.fn();
    const unsub = subscribeDropMode(heard);

    expect(dropModeForDrag()).toBe('copy');
    key('keydown', { metaKey: true, key: 'Meta' });
    expect(dropModeForDrag()).toBe('move');
    expect(heard).toHaveBeenCalledOnce();

    key('keydown', { metaKey: true, key: 'a' });
    expect(heard).toHaveBeenCalledOnce();

    unsub();
    key('keyup', { metaKey: false, key: 'Meta' });
    expect(dropModeForDrag()).toBe('copy');
    expect(heard).toHaveBeenCalledOnce();
  });

  it('uses Meta on Apple and ignores Ctrl', () => {
    setUserAgent('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)');
    key('keydown', { ctrlKey: true, metaKey: false, key: 'Control' });
    expect(dropModeForDrag()).toBe('copy');
    key('keyup', { key: 'Meta', metaKey: true });
    expect(dropModeForDrag()).toBe('move');
    key('keyup', { key: 'Meta', metaKey: false });
    expect(dropModeForDrag()).toBe('copy');
  });

  it('uses Ctrl off Apple and ignores Meta', () => {
    setUserAgent('Mozilla/5.0 (Windows NT 10.0; Win64; x64)');
    key('keydown', { metaKey: true, ctrlKey: false, key: 'Meta' });
    expect(dropModeForDrag()).toBe('copy');
    key('keydown', { ctrlKey: true, metaKey: false, key: 'Control' });
    expect(dropModeForDrag()).toBe('move');
    key('keyup', { ctrlKey: false, key: 'Control' });
    expect(dropModeForDrag()).toBe('copy');
  });

  it('reads the modifier from mousedown and mousemove', () => {
    setUserAgent('Mozilla/5.0 (Windows NT 10.0; Win64; x64)');
    mouse('mousedown', { ctrlKey: true });
    expect(dropModeForDrag()).toBe('move');
    mouse('mousemove', { ctrlKey: false });
    expect(dropModeForDrag()).toBe('copy');
  });

  it('clears move mode when the window blurs', () => {
    setUserAgent('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)');
    key('keydown', { metaKey: true, key: 'Meta' });
    expect(dropModeForDrag()).toBe('move');
    window.dispatchEvent(new Event('blur'));
    expect(dropModeForDrag()).toBe('copy');
  });
});
