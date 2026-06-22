// WebAuthn browser ceremony glue (Phase 134 / ADR-040).
//
// Converts the BFF's (go-webauthn) base64url ceremony options into the
// ArrayBuffer-typed options the browser's `navigator.credentials` API expects,
// runs the registration ceremony, and re-encodes the authenticator's response
// into the standard WebAuthn JSON the BFF parses. The cryptography is the
// platform authenticator's + the server library's; this is only encoding glue.

import * as authApi from './auth';

function b64urlToBuf(s: string): ArrayBuffer {
  const pad = s.length % 4 === 0 ? '' : '='.repeat(4 - (s.length % 4));
  const b64 = (s + pad).replace(/-/g, '+').replace(/_/g, '/');
  const bin = atob(b64);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out.buffer;
}

function bufToB64url(buf: ArrayBuffer): string {
  const bytes = new Uint8Array(buf);
  let bin = '';
  for (const b of bytes) bin += String.fromCharCode(b);
  return btoa(bin).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

// Failure is reported as a stable CODE, not a message: the UI localizes it
// (the user reported the raw English/lowercase backend message leaking through).
//   unsupported — this browser has no WebAuthn
//   cancelled   — the user dismissed the OS prompt, or it timed out
//   failed      — begin/finish round-trip failed (e.g. origin / RP-ID mismatch)
export type CeremonyErrorCode = 'unsupported' | 'cancelled' | 'failed';
type CeremonyResult = { ok: true } | { ok: false; code: CeremonyErrorCode };

/** Registers a new passkey for the current user via the full ceremony. */
export async function registerPasskey(): Promise<CeremonyResult> {
  if (typeof navigator === 'undefined' || !navigator.credentials) {
    return { ok: false, code: 'unsupported' };
  }

  const begin = await authApi.passkeyRegisterBegin();
  if (!begin.ok) return { ok: false, code: 'failed' };

  // go-webauthn wraps the options under `publicKey`.
  const opts = (begin.data as { publicKey?: Record<string, unknown> }).publicKey;
  if (!opts) return { ok: false, code: 'failed' };

  const user = opts.user as { id: string; name: string; displayName: string };
  const exclude = (opts.excludeCredentials as Array<Record<string, unknown>> | undefined) ?? [];

  const publicKey: PublicKeyCredentialCreationOptions = {
    ...(opts as unknown as PublicKeyCredentialCreationOptions),
    challenge: b64urlToBuf(opts.challenge as string),
    user: { ...user, id: b64urlToBuf(user.id) },
    excludeCredentials: exclude.map((c) => ({
      ...(c as unknown as PublicKeyCredentialDescriptor),
      id: b64urlToBuf(c.id as string)
    }))
  };

  let credential: PublicKeyCredential;
  try {
    const c = await navigator.credentials.create({ publicKey });
    if (!c) return { ok: false, code: 'cancelled' };
    credential = c as PublicKeyCredential;
  } catch {
    return { ok: false, code: 'cancelled' };
  }

  const att = credential.response as AuthenticatorAttestationResponse;
  const body = {
    id: credential.id,
    rawId: bufToB64url(credential.rawId),
    type: credential.type,
    response: {
      clientDataJSON: bufToB64url(att.clientDataJSON),
      attestationObject: bufToB64url(att.attestationObject)
    },
    clientExtensionResults: credential.getClientExtensionResults?.() ?? {}
  };

  const finish = await authApi.passkeyRegisterFinish(body);
  return finish.ok ? { ok: true } : { ok: false, code: 'failed' };
}
