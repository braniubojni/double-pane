import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { PathBar } from './PathBar';

vi.mock('../../entities/file/queries', () => ({
  usePathCompletions: () => ({ data: [] }),
}));
vi.mock('../../shared/ui/SnackbarHost', () => ({
  useSnack: (sel: (s: { show: () => void }) => unknown) => sel({ show: vi.fn() }),
}));

const setup = (path = '/home/a') => {
  const props = {
    paneId: 'left' as const,
    path,
    onNavigate: vi.fn(),
    onUp: vi.fn(),
    onHome: vi.fn(),
    onFocusPane: vi.fn(),
  };
  const view = render(<PathBar {...props} />);
  const input = () => screen.getByRole('combobox') as HTMLInputElement;
  const rerender = (p: string) => view.rerender(<PathBar {...props} path={p} />);
  return { props, input, rerender };
};

describe('PathBar', () => {
  it('shows breadcrumbs and path in input when idle', () => {
    const { input } = setup();
    expect(screen.getByTestId('path-crumbs-left')).toBeTruthy();
    expect(input().value).toBe('/home/a');
  });

  it('hides breadcrumbs on focus and lets user type', () => {
    const { input, props } = setup();
    fireEvent.focus(input());
    expect(screen.queryByTestId('path-crumbs-left')).toBeNull();
    expect(props.onFocusPane).toHaveBeenCalled();
    fireEvent.change(input(), { target: { value: '/tmp' } });
    expect(input().value).toBe('/tmp');
  });

  it('Enter navigates with trailing slashes stripped', () => {
    const { input, props } = setup();
    fireEvent.focus(input());
    fireEvent.change(input(), { target: { value: '/tmp/x///' } });
    fireEvent.keyDown(input(), { key: 'Enter' });
    expect(props.onNavigate).toHaveBeenCalledWith('/tmp/x');
    expect(screen.getByTestId('path-crumbs-left')).toBeTruthy();
  });

  it('Escape reverts draft and leaves edit mode', () => {
    const { input, props } = setup();
    fireEvent.focus(input());
    fireEvent.change(input(), { target: { value: '/tmp' } });
    fireEvent.keyDown(input(), { key: 'Escape' });
    expect(input().value).toBe('/home/a');
    expect(screen.getByTestId('path-crumbs-left')).toBeTruthy();
    expect(props.onNavigate).not.toHaveBeenCalled();
  });

  it('path prop change resets draft and exits edit mode', () => {
    const { input, rerender } = setup();
    fireEvent.focus(input());
    fireEvent.change(input(), { target: { value: '/tmp' } });
    rerender('/home/b');
    expect(input().value).toBe('/home/b');
    expect(screen.getByTestId('path-crumbs-left')).toBeTruthy();
  });

  it('same path rerender keeps in-progress edit', () => {
    const { input, rerender } = setup();
    fireEvent.focus(input());
    fireEvent.change(input(), { target: { value: '/tmp' } });
    rerender('/home/a');
    expect(input().value).toBe('/tmp');
    expect(screen.queryByTestId('path-crumbs-left')).toBeNull();
  });
});
