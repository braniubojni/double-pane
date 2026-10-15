import { render } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { usePaneStore } from '../../../features/pane/paneStore';
import { historyNav } from '../../../widgets/file-pane/helpers';
import { useMouseNavButtons } from './useMouseNavButtons';

vi.mock('../../../widgets/file-pane/helpers', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../../widgets/file-pane/helpers')>();
  return { ...actual, historyNav: vi.fn(() => true) };
});

const Harness = () => {
  useMouseNavButtons();
  return null;
};

const click = (button: number, target: EventTarget = window, type = 'mousedown') => {
  const event = new MouseEvent(type, { button, bubbles: true, cancelable: true });
  target.dispatchEvent(event);
  return event;
};

afterEach(() => {
  document.body.replaceChildren();
  vi.mocked(historyNav).mockClear();
});

describe('useMouseNavButtons', () => {
  it('goes back on button 3 and forward on button 4 for the active pane', () => {
    usePaneStore.setState({ activePane: 'right' });
    render(<Harness />);

    const back = click(3);
    const forward = click(4, window, 'auxclick');

    expect(back.defaultPrevented).toBe(true);
    expect(forward.defaultPrevented).toBe(true);
    expect(historyNav).toHaveBeenNthCalledWith(1, 'right', 'back');
    expect(historyNav).toHaveBeenNthCalledWith(2, 'right', 'forward');
  });

  it('ignores primary, middle, and right clicks', () => {
    render(<Harness />);
    click(0);
    click(1);
    click(2);
    expect(historyNav).not.toHaveBeenCalled();
  });

  it('ignores back/forward while the target is editable', () => {
    const input = document.createElement('input');
    const dialog = document.createElement('div');
    dialog.setAttribute('role', 'dialog');
    const inside = document.createElement('span');
    dialog.append(inside);
    document.body.append(input, dialog);
    render(<Harness />);

    click(3, input);
    click(4, inside, 'auxclick');

    expect(historyNav).not.toHaveBeenCalled();
  });

  it('stops listening after unmount', () => {
    const view = render(<Harness />);
    view.unmount();
    click(3);
    expect(historyNav).not.toHaveBeenCalled();
  });
});
