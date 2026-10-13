import { afterEach, describe, expect, it } from 'vitest';
import { getPaneGrid, isEditableTarget } from './helpers';

afterEach(() => {
  document.body.replaceChildren();
});

describe('getPaneGrid', () => {
  it('returns the grid element for that pane', () => {
    const left = document.createElement('div');
    left.setAttribute('data-pane-grid', 'left');
    const right = document.createElement('div');
    right.setAttribute('data-pane-grid', 'right');
    document.body.append(left, right);

    expect(getPaneGrid('left')).toBe(left);
    expect(getPaneGrid('right')).toBe(right);
  });

  it('returns null when the pane grid is not mounted', () => {
    expect(getPaneGrid('left')).toBeNull();
  });
});

describe('isEditableTarget', () => {
  it('is false for null and a plain element', () => {
    expect(isEditableTarget(null)).toBe(false);
    expect(isEditableTarget(document.createElement('div'))).toBe(false);
  });

  it('is true for inputs, textareas, and contenteditable', () => {
    expect(isEditableTarget(document.createElement('input'))).toBe(true);
    expect(isEditableTarget(document.createElement('textarea'))).toBe(true);
    const rich = document.createElement('div');
    Object.defineProperty(rich, 'isContentEditable', { value: true });
    document.body.append(rich);
    expect(isEditableTarget(rich)).toBe(true);
  });

  it('is true inside a terminal, dialog, popover, or menu', () => {
    const xterm = document.createElement('div');
    xterm.className = 'xterm';
    const cell = document.createElement('span');
    xterm.append(cell);

    const dialog = document.createElement('div');
    dialog.setAttribute('role', 'dialog');
    const field = document.createElement('span');
    dialog.append(field);

    const popover = document.createElement('div');
    popover.className = 'MuiPopover-root';
    const popChild = document.createElement('span');
    popover.append(popChild);

    const menu = document.createElement('div');
    menu.className = 'MuiMenu-root';
    const menuChild = document.createElement('span');
    menu.append(menuChild);

    expect(isEditableTarget(cell)).toBe(true);
    expect(isEditableTarget(field)).toBe(true);
    expect(isEditableTarget(popChild)).toBe(true);
    expect(isEditableTarget(menuChild)).toBe(true);
  });
});
