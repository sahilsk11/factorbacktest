import {
  ArrowLeft,
  Check,
  KeyRound,
  LoaderCircle,
  Mail,
  MessageSquareText,
  ShieldCheck,
  X,
} from 'lucide-react';
import { useEffect, useState, type FormEventHandler } from 'react';
import { createPortal } from 'react-dom';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { isApiError } from '@/lib/api';
import { useAuth } from '@/lib/auth-context';
import { cn } from '@/lib/utils';

type AuthMethod = 'email' | 'sms';
type AuthStep = 'request' | 'verify';
type LoadingAction = 'google' | 'request' | 'verify' | null;

interface PendingChallenge {
  method: AuthMethod;
  target: string;
  display: string;
}

const methodLabels: Record<AuthMethod, { label: string; icon: typeof Mail; cta: string }> = {
  email: { label: 'Email', icon: Mail, cta: 'Email code' },
  sms: { label: 'SMS', icon: MessageSquareText, cta: 'Text code' },
};

export function AuthModal({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}): React.ReactNode {
  const { signIn } = useAuth();
  const [method, setMethod] = useState<AuthMethod>('email');
  const [step, setStep] = useState<AuthStep>('request');
  const [email, setEmail] = useState('');
  const [phoneNumber, setPhoneNumber] = useState('');
  const [code, setCode] = useState('');
  const [pending, setPending] = useState<PendingChallenge | null>(null);
  const [loading, setLoading] = useState<LoadingAction>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };

    window.addEventListener('keydown', onKeyDown);
    return () => {
      window.removeEventListener('keydown', onKeyDown);
    };
  }, [onClose, open]);

  if (!open) return null;

  const selected = methodLabels[method];
  const Icon = selected.icon;
  const normalizedEmail = email.trim().toLowerCase();
  const normalizedPhone = normalizePhoneNumber(phoneNumber);
  const canRequest =
    loading === null &&
    (method === 'email' ? isValidEmail(normalizedEmail) : isValidE164(normalizedPhone));
  const canVerify = loading === null && code.length === 6 && pending !== null;

  const startGoogle = () => {
    if (loading !== null) return;
    setError(null);
    setLoading('google');
    void signIn.google().catch((err: unknown) => {
      setLoading(null);
      setError(authErrorMessage(err, 'Google sign-in failed'));
    });
  };

  const requestCode = async () => {
    if (!canRequest) return;
    setError(null);
    setLoading('request');

    const target = method === 'email' ? normalizedEmail : normalizedPhone;
    try {
      if (method === 'email') {
        await signIn.sendEmailCode(target);
      } else {
        await signIn.sendSmsCode(target);
      }
      setPending({
        method,
        target,
        display: method === 'email' ? target : maskPhone(target),
      });
      setCode('');
      setStep('verify');
    } catch (err: unknown) {
      setError(authErrorMessage(err, 'Could not send a sign-in code'));
    } finally {
      setLoading(null);
    }
  };

  const verifyCode = async () => {
    if (!pending || code.length !== 6) return;
    setError(null);
    setLoading('verify');

    try {
      const user =
        pending.method === 'email'
          ? await signIn.verifyEmailCode(pending.target, code)
          : await signIn.verifySmsCode(pending.target, code);

      if (!user) {
        setError(
          'Code verified, but this browser could not attach the session cookie. This can happen when localhost uses the production API; use factor.trade or a local API for a persistent session.',
        );
        return;
      }

      onClose();
    } catch (err: unknown) {
      setError(authErrorMessage(err, 'That code did not match'));
    } finally {
      setLoading(null);
    }
  };

  const onRequestSubmit: FormEventHandler<HTMLFormElement> = (event) => {
    event.preventDefault();
    void requestCode();
  };

  const onVerifySubmit: FormEventHandler<HTMLFormElement> = (event) => {
    event.preventDefault();
    void verifyCode();
  };

  return createPortal(
    <div
      className="fixed inset-0 z-[9999] overflow-y-auto bg-background/85 backdrop-blur-xl"
      role="presentation"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div className="flex min-h-full items-center justify-center px-4 py-10">
        <section
          role="dialog"
          aria-modal="true"
          aria-labelledby="auth-modal-title"
          className="relative w-full max-w-[460px] overflow-hidden rounded-lg border border-border-strong bg-card text-foreground shadow-[0_28px_90px_rgba(0,0,0,0.62)]"
        >
          <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-accent to-transparent" />

          <div className="p-6 sm:p-7">
            <button
              type="button"
              className="absolute top-4 right-4 inline-flex size-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-elevated hover:text-foreground"
              aria-label="Close sign-in"
              onClick={onClose}
            >
              <X className="size-4" aria-hidden />
            </button>

            <div className="mb-6 pr-10">
              <div className="mb-4 flex size-10 items-center justify-center rounded-md border border-border bg-elevated text-gain">
                <ShieldCheck className="size-5" aria-hidden />
              </div>
              <h2 id="auth-modal-title" className="text-2xl font-semibold tracking-tight">
                Sign in to Factor
              </h2>
              <p className="mt-2 text-sm leading-6 text-muted-foreground">
                Use Google, email, or SMS to open a secure session.
              </p>
            </div>

            {step === 'request' ? (
              <div className="space-y-5">
                <button
                  type="button"
                  className={cn(
                    'group flex h-12 w-full items-center justify-between rounded-md border border-border bg-elevated/60 px-4 text-sm font-medium text-foreground',
                    'transition-[border-color,background-color,transform] duration-150 ease-out',
                    'hover:-translate-y-px hover:border-border-strong hover:bg-elevated',
                    'disabled:pointer-events-none disabled:opacity-60',
                  )}
                  disabled={loading !== null}
                  onClick={startGoogle}
                >
                  <span className="flex items-center gap-3">
                    <span className="grid size-7 place-items-center rounded-md bg-foreground text-sm font-semibold text-background">
                      G
                    </span>
                    Sign in with Google
                  </span>
                  {loading === 'google' ? (
                    <LoaderCircle
                      className="size-4 animate-spin text-muted-foreground"
                      aria-hidden
                    />
                  ) : (
                    <span className="font-mono text-xs text-muted-foreground">OAuth</span>
                  )}
                </button>

                <div className="flex items-center gap-3">
                  <div className="h-px flex-1 bg-border" />
                  <span className="text-xs font-medium uppercase tracking-widest text-subtle-foreground">
                    or code
                  </span>
                  <div className="h-px flex-1 bg-border" />
                </div>

                <div className="grid grid-cols-2 gap-2 rounded-md border border-border bg-background/55 p-1">
                  {(['email', 'sms'] as const).map((option) => {
                    const OptionIcon = methodLabels[option].icon;
                    const active = method === option;
                    return (
                      <button
                        key={option}
                        type="button"
                        className={cn(
                          'flex h-10 items-center justify-center gap-2 rounded-sm text-sm font-medium transition-colors',
                          active
                            ? 'bg-elevated text-foreground'
                            : 'text-muted-foreground hover:bg-elevated/60 hover:text-foreground',
                        )}
                        onClick={() => {
                          setMethod(option);
                          setError(null);
                        }}
                      >
                        <OptionIcon className="size-4" aria-hidden />
                        {methodLabels[option].label}
                      </button>
                    );
                  })}
                </div>

                <form className="space-y-4" onSubmit={onRequestSubmit}>
                  <label className="block">
                    <span className="mb-2 flex items-center gap-2 text-xs font-medium uppercase tracking-widest text-subtle-foreground">
                      <Icon className="size-3.5" aria-hidden />
                      {method === 'email' ? 'Email address' : 'Mobile number'}
                    </span>
                    {method === 'email' ? (
                      <Input
                        autoComplete="email"
                        inputMode="email"
                        placeholder="you@example.com"
                        type="email"
                        value={email}
                        onChange={(event) => {
                          setEmail(event.target.value);
                          setError(null);
                        }}
                      />
                    ) : (
                      <Input
                        autoComplete="tel"
                        inputMode="tel"
                        placeholder="(415) 555-0134"
                        type="tel"
                        value={phoneNumber}
                        onChange={(event) => {
                          setPhoneNumber(event.target.value);
                          setError(null);
                        }}
                      />
                    )}
                  </label>

                  <Button type="submit" className="h-11 w-full" disabled={!canRequest}>
                    {loading === 'request' ? (
                      <LoaderCircle className="size-4 animate-spin" aria-hidden />
                    ) : (
                      <KeyRound className="size-4" aria-hidden />
                    )}
                    {selected.cta}
                  </Button>
                </form>
              </div>
            ) : (
              <form className="space-y-5" onSubmit={onVerifySubmit}>
                <button
                  type="button"
                  className="inline-flex items-center gap-2 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
                  onClick={() => {
                    setStep('request');
                    setCode('');
                    setError(null);
                  }}
                >
                  <ArrowLeft className="size-4" aria-hidden />
                  Change method
                </button>

                <div className="rounded-lg border border-border bg-background/55 p-4">
                  <p className="text-xs font-medium uppercase tracking-widest text-subtle-foreground">
                    Code sent
                  </p>
                  <p className="mt-1 font-mono text-sm text-foreground">{pending?.display}</p>
                </div>

                <label className="block">
                  <span className="mb-2 block text-xs font-medium uppercase tracking-widest text-subtle-foreground">
                    Six-digit code
                  </span>
                  <Input
                    autoFocus
                    autoComplete="one-time-code"
                    className="h-14 text-center font-mono text-xl tracking-[0.5em]"
                    inputMode="numeric"
                    maxLength={6}
                    pattern="[0-9]*"
                    placeholder="000000"
                    value={code}
                    onChange={(event) => {
                      setCode(event.target.value.replace(/\D/g, '').slice(0, 6));
                      setError(null);
                    }}
                  />
                </label>

                <Button type="submit" className="h-11 w-full" disabled={!canVerify}>
                  {loading === 'verify' ? (
                    <LoaderCircle className="size-4 animate-spin" aria-hidden />
                  ) : (
                    <Check className="size-4" aria-hidden />
                  )}
                  Continue
                </Button>
              </form>
            )}

            {error && (
              <div className="mt-5 rounded-md border border-loss/30 bg-loss/10 px-3 py-2 text-sm text-loss">
                {error}
              </div>
            )}
          </div>
        </section>
      </div>
    </div>,
    document.body,
  );
}

function normalizePhoneNumber(input: string): string {
  const trimmed = input.trim();
  if (trimmed.startsWith('+')) {
    return `+${trimmed.replace(/\D/g, '')}`;
  }

  const digits = trimmed.replace(/\D/g, '');
  if (digits.length === 10) return `+1${digits}`;
  if (digits.length === 11 && digits.startsWith('1')) return `+${digits}`;
  return trimmed;
}

function isValidEmail(value: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);
}

function isValidE164(value: string): boolean {
  return /^\+[1-9]\d{1,14}$/.test(value);
}

function maskPhone(value: string): string {
  const digits = value.replace(/\D/g, '');
  return `+${digits.slice(0, Math.max(digits.length - 4, 1)).replace(/\d/g, '*')}${digits.slice(-4)}`;
}

function authErrorMessage(error: unknown, fallback: string): string {
  if (isApiError(error)) {
    if (error.status === 401) return 'That code did not match.';
    if (error.status === 403) return 'This browser origin is not allowed by the API.';
    if (error.status === 404) return 'This sign-in method is not available in this environment.';
    if (error.status === 503) return 'Authentication is temporarily unavailable.';
    return error.message;
  }

  return error instanceof Error ? error.message : fallback;
}
