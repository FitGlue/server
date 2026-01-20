/**
 * HTTP Error utilities for consistent error handling across the application.
 *
 * Provides:
 * - HttpError class with status code and response body
 * - parseErrorResponse helper for explicit error handling
 * - errorLoggingMiddleware for openapi-fetch clients
 */

import type { Middleware } from 'openapi-fetch';
import type { Logger } from 'winston';

/** Maximum size of error body to include in error messages */
export const MAX_ERROR_BODY_SIZE = 500;

/**
 * Truncate a string to maxLen, adding "..." if truncated.
 */
function truncate(s: string, maxLen: number): string {
  if (s.length <= maxLen) return s;
  return s.substring(0, maxLen) + '...';
}

/**
 * Custom Error class for HTTP errors with status code and response body.
 */
export class HttpError extends Error {
  public readonly status: number;
  public readonly statusText: string;
  public readonly body: string;
  public readonly url?: string;

  constructor(status: number, statusText: string, body: string, url?: string) {
    const truncatedBody = truncate(body, MAX_ERROR_BODY_SIZE);
    const message = truncatedBody
      ? `${statusText} (${status}): ${truncatedBody}`
      : `${statusText} (${status})`;

    super(message);
    this.name = 'HttpError';
    this.status = status;
    this.statusText = statusText;
    this.body = truncatedBody;
    this.url = url;
  }
}

/**
 * Parse a fetch Response and return an HttpError if it's an error response.
 * Returns null for success responses.
 *
 * @param response - The fetch Response to check
 * @param body - Optional pre-read body (if not provided, will clone and read)
 */
export async function parseErrorResponse(response: Response, body?: string): Promise<HttpError | null> {
  if (response.ok) return null;

  const errorBody = body ?? await response.clone().text();
  return new HttpError(
    response.status,
    response.statusText || 'Error',
    errorBody,
    response.url
  );
}

/**
 * Middleware for openapi-fetch clients that logs HTTP error responses.
 *
 * @param logger - Winston logger instance for structured logging
 * @param component - Optional component name for log context (e.g., 'hevy-client')
 */
export function errorLoggingMiddleware(logger: Logger, component?: string): Middleware {
  return {
    async onResponse({ response, request }) {
      if (!response.ok) {
        const body = await response.clone().text();
        const truncatedBody = truncate(body, MAX_ERROR_BODY_SIZE);

        logger.error('HTTP error response', {
          component: component || 'http-client',
          url: request.url,
          method: request.method,
          status: response.status,
          statusText: response.statusText,
          body: truncatedBody
        });
      }
      return response;
    }
  };
}

/**
 * Wrap a fetch function with error logging.
 * Use this for custom fetch wrappers (like OAuth clients).
 *
 * @param fetchFn - The underlying fetch function
 * @param logger - Console or winston logger
 * @param provider - Provider name for context (e.g., 'strava')
 */
export function wrapFetchWithErrorLogging(
  fetchFn: typeof fetch,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  logger: { error: (msg: string, ...args: any[]) => void },
  provider: string
): typeof fetch {
  return async (input, init) => {
    const response = await fetchFn(input, init);

    if (!response.ok) {
      const body = await response.clone().text();
      const truncatedBody = truncate(body, MAX_ERROR_BODY_SIZE);

      logger.error(`[${provider}] HTTP ${response.status}: ${truncatedBody}`);
    }

    return response;
  };
}
