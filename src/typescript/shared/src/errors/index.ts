/**
 * FitGlue Errors
 *
 * Structured error types for consistent error handling across TypeScript services.
 * This module mirrors the Go errors package (pkg/errors/errors.go).
 *
 * @example
 * ```typescript
 * import { FitGlueError, ErrUserNotFound, ErrorCode } from '@fitglue/shared';
 *
 * // Use a pre-defined error
 * throw ErrUserNotFound;
 *
 * // Wrap with context
 * throw ErrUserNotFound.withCause(originalError).withMetadata('userId', userId);
 *
 * // Create a new error
 * throw new FitGlueError(ErrorCode.VALIDATION_ERROR, 'Invalid input');
 * ```
 */

// Re-export HTTP-level errors from framework
export { HttpError } from '../framework/errors';

// Re-export domain-level errors
export { ForbiddenError } from '../domain/services/authorization';

/**
 * Error codes matching the Go implementation.
 * Use these as stable identifiers for error categorization.
 */
export enum ErrorCode {
  // User errors
  USER_NOT_FOUND = 'USER_NOT_FOUND',
  USER_UNAUTHORIZED = 'USER_UNAUTHORIZED',
  USER_FORBIDDEN = 'USER_FORBIDDEN',

  // Integration errors
  INTEGRATION_NOT_FOUND = 'INTEGRATION_NOT_FOUND',
  INTEGRATION_EXPIRED = 'INTEGRATION_EXPIRED',
  INTEGRATION_AUTH_FAILED = 'INTEGRATION_AUTH_FAILED',
  INTEGRATION_RATE_LIMITED = 'INTEGRATION_RATE_LIMITED',

  // Pipeline errors
  PIPELINE_NOT_FOUND = 'PIPELINE_NOT_FOUND',
  PIPELINE_INVALID_CONFIG = 'PIPELINE_INVALID_CONFIG',

  // Enricher errors
  ENRICHER_FAILED = 'ENRICHER_FAILED',
  ENRICHER_NOT_FOUND = 'ENRICHER_NOT_FOUND',
  ENRICHER_TIMEOUT = 'ENRICHER_TIMEOUT',
  ENRICHER_SKIPPED = 'ENRICHER_SKIPPED',

  // Activity errors
  ACTIVITY_NOT_FOUND = 'ACTIVITY_NOT_FOUND',
  ACTIVITY_INVALID_FORMAT = 'ACTIVITY_INVALID_FORMAT',

  // Infrastructure errors
  STORAGE_ERROR = 'STORAGE_ERROR',
  PUBSUB_ERROR = 'PUBSUB_ERROR',
  SECRET_ERROR = 'SECRET_ERROR',
  NOTIFICATION_ERROR = 'NOTIFICATION_ERROR',

  // General errors
  VALIDATION_ERROR = 'VALIDATION_ERROR',
  INTERNAL_ERROR = 'INTERNAL_ERROR',
  TIMEOUT_ERROR = 'TIMEOUT_ERROR',
}

/**
 * Base error class for all FitGlue errors.
 * Extends Error to work with standard try/catch and instanceof checks.
 */
export class FitGlueError extends Error {
  readonly code: ErrorCode;
  readonly retryable: boolean;
  readonly metadata: Record<string, string>;
  readonly cause?: Error;

  constructor(
    code: ErrorCode,
    message: string,
    options: {
      cause?: Error;
      retryable?: boolean;
      metadata?: Record<string, string>;
    } = {}
  ) {
    super(message);
    this.name = 'FitGlueError';
    this.code = code;
    this.retryable = options.retryable ?? false;
    this.cause = options.cause;
    this.metadata = options.metadata ?? {};

    // Maintain proper stack trace in V8
    if (Error.captureStackTrace) {
      Error.captureStackTrace(this, FitGlueError);
    }
  }

  /**
   * Returns a formatted error string: [CODE] message: cause
   */
  toString(): string {
    if (this.cause) {
      return `[${this.code}] ${this.message}: ${this.cause.message}`;
    }
    return `[${this.code}] ${this.message}`;
  }

  /**
   * Creates a new error with an underlying cause.
   */
  withCause(cause: Error): FitGlueError {
    return new FitGlueError(this.code, this.message, {
      cause,
      retryable: this.retryable,
      metadata: { ...this.metadata },
    });
  }

  /**
   * Creates a new error with a custom message.
   */
  withMessage(message: string): FitGlueError {
    return new FitGlueError(this.code, message, {
      cause: this.cause,
      retryable: this.retryable,
      metadata: { ...this.metadata },
    });
  }

  /**
   * Creates a new error with additional metadata.
   */
  withMetadata(key: string, value: string): FitGlueError {
    return new FitGlueError(this.code, this.message, {
      cause: this.cause,
      retryable: this.retryable,
      metadata: { ...this.metadata, [key]: value },
    });
  }

  /**
   * Converts to a JSON-serializable object for logging.
   */
  toJSON(): Record<string, unknown> {
    return {
      code: this.code,
      message: this.message,
      retryable: this.retryable,
      metadata: this.metadata,
      cause: this.cause?.message,
      stack: this.stack,
    };
  }
}

// ============================================================================
// Pre-defined Sentinel Errors
// Use these with instanceof or wrap with .withCause()
// ============================================================================

// User errors
export const ErrUserNotFound = new FitGlueError(ErrorCode.USER_NOT_FOUND, 'user not found');
export const ErrUserUnauthorized = new FitGlueError(ErrorCode.USER_UNAUTHORIZED, 'unauthorized');
export const ErrUserForbidden = new FitGlueError(ErrorCode.USER_FORBIDDEN, 'forbidden');

// Integration errors
export const ErrIntegrationNotFound = new FitGlueError(ErrorCode.INTEGRATION_NOT_FOUND, 'integration not found');
export const ErrIntegrationExpired = new FitGlueError(ErrorCode.INTEGRATION_EXPIRED, 'integration token expired', { retryable: true });
export const ErrIntegrationAuthFailed = new FitGlueError(ErrorCode.INTEGRATION_AUTH_FAILED, 'integration authentication failed');
export const ErrIntegrationRateLimited = new FitGlueError(ErrorCode.INTEGRATION_RATE_LIMITED, 'integration rate limited', { retryable: true });

// Pipeline errors
export const ErrPipelineNotFound = new FitGlueError(ErrorCode.PIPELINE_NOT_FOUND, 'pipeline not found');
export const ErrPipelineInvalidConfig = new FitGlueError(ErrorCode.PIPELINE_INVALID_CONFIG, 'invalid pipeline configuration');

// Enricher errors
export const ErrEnricherFailed = new FitGlueError(ErrorCode.ENRICHER_FAILED, 'enricher failed', { retryable: true });
export const ErrEnricherNotFound = new FitGlueError(ErrorCode.ENRICHER_NOT_FOUND, 'enricher not found');
export const ErrEnricherTimeout = new FitGlueError(ErrorCode.ENRICHER_TIMEOUT, 'enricher timed out', { retryable: true });
export const ErrEnricherSkipped = new FitGlueError(ErrorCode.ENRICHER_SKIPPED, 'enricher skipped');

// Activity errors
export const ErrActivityNotFound = new FitGlueError(ErrorCode.ACTIVITY_NOT_FOUND, 'activity not found');
export const ErrActivityInvalidFormat = new FitGlueError(ErrorCode.ACTIVITY_INVALID_FORMAT, 'invalid activity format');

// Infrastructure errors
export const ErrStorageError = new FitGlueError(ErrorCode.STORAGE_ERROR, 'storage error', { retryable: true });
export const ErrPubSubError = new FitGlueError(ErrorCode.PUBSUB_ERROR, 'pubsub error', { retryable: true });
export const ErrSecretError = new FitGlueError(ErrorCode.SECRET_ERROR, 'secret access error', { retryable: true });
export const ErrNotificationError = new FitGlueError(ErrorCode.NOTIFICATION_ERROR, 'notification error', { retryable: true });

// General errors
export const ErrValidation = new FitGlueError(ErrorCode.VALIDATION_ERROR, 'validation error');
export const ErrInternal = new FitGlueError(ErrorCode.INTERNAL_ERROR, 'internal error');
export const ErrTimeout = new FitGlueError(ErrorCode.TIMEOUT_ERROR, 'timeout', { retryable: true });

// ============================================================================
// Utility Functions
// ============================================================================

/**
 * Wraps an error with a FitGlueError.
 */
export function wrapError(cause: Error, code: ErrorCode, message: string): FitGlueError {
  return new FitGlueError(code, message, { cause });
}

/**
 * Wraps an error with a retryable FitGlueError.
 */
export function wrapRetryableError(cause: Error, code: ErrorCode, message: string): FitGlueError {
  return new FitGlueError(code, message, { cause, retryable: true });
}

/**
 * Checks if an error is a FitGlueError and is retryable.
 */
export function isRetryable(err: unknown): boolean {
  if (err instanceof FitGlueError) {
    return err.retryable;
  }
  return false;
}

/**
 * Extracts the error code from an error.
 * Returns INTERNAL_ERROR if not a FitGlueError.
 */
export function getErrorCode(err: unknown): ErrorCode {
  if (err instanceof FitGlueError) {
    return err.code;
  }
  return ErrorCode.INTERNAL_ERROR;
}

/**
 * Type guard for FitGlueError.
 */
export function isFitGlueError(err: unknown): err is FitGlueError {
  return err instanceof FitGlueError;
}
