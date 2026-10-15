import { act, fireEvent, render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { NameDialog } from './NameDialog';

vi.mock('@mui/material/Dialog', () => ({
  default: ({
    children,
    open,
    ...props
  }: {
    children: ReactNode;
    open: boolean;
    'data-testid'?: string;
  }) => (open ? <div {...props}>{children}</div> : null),
}));

const dispatch = vi.fn();

const renderDialog = (selectStem: boolean | undefined, name: string) =>
  render(
    <NameDialog
      testId="dialog-rename"
      title="Rename"
      label="Name"
      inputTestId="input-rename"
      confirmTestId="btn-rename"
      confirmLabel="Rename"
      state={{ open: true, name }}
      dispatch={dispatch}
      onConfirm={vi.fn()}
      selectStem={selectStem}
    />,
  );

const field = () => screen.getByRole('textbox') as HTMLInputElement;

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('NameDialog stem selection', () => {
  it('selects the stem after focus, once the timeout runs', () => {
    renderDialog(true, 'notes.txt');
    const el = field();
    fireEvent.focus(el);
    expect(el.selectionStart).toBe(el.selectionEnd);

    act(() => {
      vi.advanceTimersByTime(0);
    });
    expect(el.selectionStart).toBe(0);
    expect(el.selectionEnd).toBe('notes'.length);
  });

  it('selects up to the last dot', () => {
    renderDialog(true, 'archive.tar.gz');
    fireEvent.focus(field());
    act(() => {
      vi.advanceTimersByTime(0);
    });
    expect(field().selectionEnd).toBe('archive.tar'.length);
  });

  it('selects the whole name when there is no extension or the dot is leading', () => {
    const view = renderDialog(true, 'Makefile');
    fireEvent.focus(field());
    act(() => {
      vi.advanceTimersByTime(0);
    });
    expect(field().selectionStart).toBe(0);
    expect(field().selectionEnd).toBe('Makefile'.length);

    view.rerender(
      <NameDialog
        testId="dialog-rename"
        title="Rename"
        label="Name"
        inputTestId="input-rename"
        confirmTestId="btn-rename"
        confirmLabel="Rename"
        state={{ open: true, name: '.gitignore' }}
        dispatch={dispatch}
        onConfirm={vi.fn()}
        selectStem
      />,
    );
    fireEvent.focus(field());
    act(() => {
      vi.advanceTimersByTime(0);
    });
    expect(field().selectionEnd).toBe('.gitignore'.length);
  });

  it('leaves the caret alone when stem selection is off', () => {
    renderDialog(false, 'notes.txt');
    const el = field();
    fireEvent.focus(el);
    el.setSelectionRange(2, 2);
    act(() => {
      vi.advanceTimersByTime(0);
    });
    expect(el.selectionStart).toBe(2);
    expect(el.selectionEnd).toBe(2);
  });
});
