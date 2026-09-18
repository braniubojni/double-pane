import type { DeleteDialogAction, DeleteDialogState } from './types';

export type { DeleteDialogAction, DeleteDialogState } from './types';

export const initialDeleteDialogState: DeleteDialogState = {
  confirmOpen: false,
  permissionOpen: false,
  emptyOpen: false,
  permissionMessage: '',
  paths: [],
  permanent: false,
};

export const deleteDialogReducer = (
  state: DeleteDialogState,
  action: DeleteDialogAction,
): DeleteDialogState => {
  switch (action.type) {
    case 'open_confirm':
      return {
        ...state,
        confirmOpen: true,
        paths: action.paths,
        permanent: Boolean(action.permanent),
      };
    case 'close_confirm':
      return { ...state, confirmOpen: false, paths: [], permanent: false };
    case 'open_empty':
      return { ...state, emptyOpen: true };
    case 'close_empty':
      return { ...state, emptyOpen: false };
    case 'open_permission':
      return {
        ...state,
        confirmOpen: false,
        permissionOpen: true,
        permissionMessage: action.message,
      };
    case 'close_permission':
      return { ...state, permissionOpen: false, permissionMessage: '' };
    case 'reset':
      return { ...initialDeleteDialogState };
    default:
      return state;
  }
};
