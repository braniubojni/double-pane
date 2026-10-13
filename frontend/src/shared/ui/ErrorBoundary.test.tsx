import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ErrorBoundary, GlobalErrorHost } from './ErrorBoundary';

const Bomb = ({ message }: { message: string }) => {
  throw new Error(message);
};

const reject = (reason: unknown) => {
  act(() => {
    const event = new Event('unhandledrejection') as PromiseRejectionEvent;
    Object.defineProperty(event, 'reason', { value: reason });
    window.dispatchEvent(event);
  });
};

afterEach(() => {
  vi.restoreAllMocks();
});

describe('ErrorBoundary', () => {
  it('shows the render error and reloads from the refresh button', () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
    render(
      <ErrorBoundary>
        <Bomb message="render blew up" />
      </ErrorBoundary>,
    );

    expect(screen.getByTestId('global-error-recovery').textContent).toContain('render blew up');
    expect(screen.queryByTestId('btn-error-dismiss')).toBeNull();
    fireEvent.click(screen.getByTestId('btn-error-refresh'));
    expect(screen.getByText('render blew up')).toBeTruthy();
  });

  it('renders children when nothing throws', () => {
    render(
      <ErrorBoundary>
        <span>ok</span>
      </ErrorBoundary>,
    );
    expect(screen.getByText('ok')).toBeTruthy();
  });
});

describe('GlobalErrorHost', () => {
  it('shows window errors except script noise and ResizeObserver', () => {
    render(
      <GlobalErrorHost>
        <span>child</span>
      </GlobalErrorHost>,
    );
    expect(screen.getByText('child')).toBeTruthy();

    act(() => {
      window.dispatchEvent(new ErrorEvent('error', { message: 'Script error.' }));
      window.dispatchEvent(
        new ErrorEvent('error', { message: 'ResizeObserver loop limit exceeded' }),
      );
      window.dispatchEvent(new ErrorEvent('error', { message: '' }));
    });
    expect(screen.getByText('Unexpected error')).toBeTruthy();
  });

  it('shows a named window error and dismisses it', () => {
    render(
      <GlobalErrorHost>
        <span>child</span>
      </GlobalErrorHost>,
    );
    act(() => {
      window.dispatchEvent(new ErrorEvent('error', { message: 'disk full' }));
    });
    expect(screen.getByText('disk full')).toBeTruthy();
    fireEvent.click(screen.getByTestId('btn-error-dismiss'));
    expect(screen.getByText('child')).toBeTruthy();
  });

  it('surfaces rejections, including the DataGrid selection crash', () => {
    render(
      <GlobalErrorHost>
        <span>child</span>
      </GlobalErrorHost>,
    );
    reject(new Error('rowSelectionModel can only contain 1 item'));
    expect(screen.getByText('rowSelectionModel can only contain 1 item')).toBeTruthy();
  });

  it('surfaces string reasons and non-error reasons', () => {
    const view = render(
      <GlobalErrorHost>
        <span>child</span>
      </GlobalErrorHost>,
    );
    reject('plain failure');
    expect(screen.getByText('plain failure')).toBeTruthy();
    view.unmount();

    render(
      <GlobalErrorHost>
        <span>child</span>
      </GlobalErrorHost>,
    );
    reject({ code: 1 });
    expect(screen.getByText('Unhandled promise rejection')).toBeTruthy();
  });

  it('logs fetch noise and leaves the children up', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined);
    render(
      <GlobalErrorHost>
        <span>child</span>
      </GlobalErrorHost>,
    );
    reject(new Error('Failed to fetch'));
    reject('Load failed');
    expect(screen.getByText('child')).toBeTruthy();
    const logged = warn.mock.calls.filter((call) =>
      String(call[0]).includes('Unhandled rejection'),
    );
    expect(logged).toHaveLength(2);
    warn.mockRestore();
  });

  it('stops listening after unmount', () => {
    const view = render(
      <GlobalErrorHost>
        <span>child</span>
      </GlobalErrorHost>,
    );
    view.unmount();
    window.dispatchEvent(new ErrorEvent('error', { message: 'too late' }));
    expect(screen.queryByText('too late')).toBeNull();
  });

  it('reloads from the refresh button', () => {
    render(
      <GlobalErrorHost>
        <span>child</span>
      </GlobalErrorHost>,
    );
    act(() => {
      window.dispatchEvent(new ErrorEvent('error', { message: 'disk full' }));
    });
    fireEvent.click(screen.getByTestId('btn-error-refresh'));
    expect(screen.getByText('disk full')).toBeTruthy();
  });
});
