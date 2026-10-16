import { render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Preview } from '../../entities/file/types';
import { QuickViewBody } from './QuickViewBody';

vi.mock('./QuickViewText', () => ({
  QuickViewText: ({ text }: { text: string }) => <div data-testid="text">{text}</div>,
}));

const create = vi.fn();
const revoke = vi.fn();

const preview = (patch: Partial<Preview>): Preview => ({ kind: 'image', ...patch }) as Preview;
const b64 = btoa('abc');

beforeEach(() => {
  let n = 0;
  create.mockReset().mockImplementation(() => `blob:${++n}`);
  revoke.mockReset();
  URL.createObjectURL = create;
  URL.revokeObjectURL = revoke;
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('QuickViewBody object url', () => {
  it('renders an image from a blob url', () => {
    render(
      <QuickViewBody
        path="/a.png"
        preview={preview({ data: b64, mime: 'image/png' })}
        onOpen={vi.fn()}
      />,
    );
    expect(screen.getByTestId('quick-view-image').getAttribute('src')).toBe('blob:1');
    expect((create.mock.calls[0][0] as Blob).type).toBe('image/png');
  });

  it('renders a pdf iframe with a default pdf mime', () => {
    render(
      <QuickViewBody
        path="/a.pdf"
        preview={preview({ kind: 'pdf', data: b64 })}
        onOpen={vi.fn()}
      />,
    );
    expect(screen.getByTestId('quick-view-pdf').getAttribute('src')).toBe('blob:1');
    expect((create.mock.calls[0][0] as Blob).type).toBe('application/pdf');
  });

  it('renders an empty body when image has no bytes', () => {
    render(<QuickViewBody path="/a.png" preview={preview({ data: '' })} onOpen={vi.fn()} />);
    expect(screen.queryByTestId('quick-view-image')).toBeNull();
    expect(create).not.toHaveBeenCalled();
  });

  it('does not create a url for text or message previews', () => {
    const view = render(
      <QuickViewBody
        path="/a.txt"
        preview={preview({ kind: 'text', text: 'hi' })}
        onOpen={vi.fn()}
      />,
    );
    expect(screen.getByTestId('text').textContent).toBe('hi');
    view.rerender(
      <QuickViewBody
        path="/a.bin"
        preview={preview({ kind: 'none', message: 'nope' })}
        onOpen={vi.fn()}
      />,
    );
    expect(screen.getByTestId('quick-view-message')).toBeTruthy();
    expect(create).not.toHaveBeenCalled();
  });

  it('revokes the old url and creates a new one when the payload changes', () => {
    const view = render(
      <QuickViewBody path="/a.png" preview={preview({ data: b64 })} onOpen={vi.fn()} />,
    );
    view.rerender(
      <QuickViewBody path="/b.png" preview={preview({ data: btoa('xyz') })} onOpen={vi.fn()} />,
    );
    expect(revoke).toHaveBeenCalledWith('blob:1');
    expect(screen.getByTestId('quick-view-image').getAttribute('src')).toBe('blob:2');
  });

  it('drops the url when switching to a non-binary preview', () => {
    const view = render(
      <QuickViewBody path="/a.png" preview={preview({ data: b64 })} onOpen={vi.fn()} />,
    );
    view.rerender(
      <QuickViewBody
        path="/a.txt"
        preview={preview({ kind: 'text', text: 't' })}
        onOpen={vi.fn()}
      />,
    );
    expect(revoke).toHaveBeenCalledWith('blob:1');
    expect(screen.getByTestId('text')).toBeTruthy();
  });

  it('revokes the url on unmount', () => {
    const view = render(
      <QuickViewBody path="/a.png" preview={preview({ data: b64 })} onOpen={vi.fn()} />,
    );
    view.unmount();
    expect(revoke).toHaveBeenCalledWith('blob:1');
  });
});
