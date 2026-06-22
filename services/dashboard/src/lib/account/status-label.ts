// Localized label for an account/user status value (active / invited /
// suspended), shared by the account identity card and the admin user table.
// Falls back to the raw value for any status the UI does not yet know about.
// Relative `m` import so the helper also loads under the node-env unit tests.
import { m } from '../paraglide/messages.js';

export function statusLabel(status: string | undefined | null): string {
  switch (status) {
    case 'active':
      return m.account_status_active();
    case 'invited':
      return m.account_status_invited();
    case 'suspended':
      return m.account_status_suspended();
    default:
      return status ?? '—';
  }
}
