
export class HttpError extends Error {
  public readonly statusCode: number;
  public readonly details?: Record<string, unknown>;

  constructor(statusCode: number, message: string, details?: Record<string, unknown>) {
    super(message);
    this.statusCode = statusCode;
    this.details = details;

    // Restore prototype chain for instanceof checks
    Object.setPrototypeOf(this, HttpError.prototype);
    this.name = 'HttpError';
  }
}
