import * as Sentry from '@sentry/node';
import type { Logger } from 'winston';

export interface SentryConfig {
  dsn?: string;
  environment: string;
  release?: string;
  serverName?: string;
  tracesSampleRate?: number;
  profilesSampleRate?: number;
}

/**
 * Initialize Sentry for Cloud Functions.
 * Safe to call multiple times - will only initialize once.
 */
export function initSentry(config: SentryConfig, logger?: Logger): void {
  if (!config.dsn) {
    logger?.warn('Sentry DSN not configured - error tracking disabled');
    return;
  }

  try {
    const { dsn, environment, release, serverName, tracesSampleRate, profilesSampleRate } = config;

    Sentry.init({
      dsn,
      environment,
      release,
      serverName,
      tracesSampleRate: tracesSampleRate ?? 0.1,
      profilesSampleRate: profilesSampleRate ?? 0.1,
      integrations: [
        Sentry.httpIntegration(),
        Sentry.nativeNodeFetchIntegration(),
        Sentry.onUncaughtExceptionIntegration(),
        Sentry.onUnhandledRejectionIntegration(),
        Sentry.modulesIntegration(),
        Sentry.contextLinesIntegration(),
        Sentry.localVariablesIntegration(),
      ],
      beforeSend(event, _hint) {
        // Filter out sensitive data
        if (event.request?.headers) {
          delete event.request.headers['authorization'];
          delete event.request.headers['cookie'];
        }
        return event;
      },
    });

    logger?.info('Sentry initialized', {
      environment: config.environment,
      release: config.release,
    });
  } catch (error) {
    logger?.error('Failed to initialize Sentry', { error });
  }
}

/**
 * Capture an exception in Sentry with additional context.
 */
export function captureException(
  error: Error,
  context?: Record<string, unknown>,
  logger?: Logger
): void {
  try {
    if (context) {
      Sentry.setContext('additional', context);
    }
    Sentry.captureException(error);
    logger?.debug('Exception captured in Sentry', { error: error.message });
  } catch (err) {
    logger?.error('Failed to capture exception in Sentry', { error: err });
  }
}

/**
 * Capture a message in Sentry.
 */
export function captureMessage(
  message: string,
  level: Sentry.SeverityLevel = 'info',
  context?: Record<string, unknown>,
  logger?: Logger
): void {
  try {
    if (context) {
      Sentry.setContext('additional', context);
    }
    Sentry.captureMessage(message, level);
    logger?.debug('Message captured in Sentry', { message, level });
  } catch (err) {
    logger?.error('Failed to capture message in Sentry', { error: err });
  }
}

/**
 * Wrap an async Cloud Function handler with Sentry error tracking.
 */
export function wrapHandler<T extends (...args: unknown[]) => Promise<unknown>>(
  handler: T,
  logger?: Logger
): T {
  return (async (...args: Parameters<T>) => {
    try {
      return await handler(...args);
    } catch (error) {
      if (error instanceof Error) {
        captureException(error, undefined, logger);
      }
      throw error;
    }
  }) as T;
}

export { Sentry };
