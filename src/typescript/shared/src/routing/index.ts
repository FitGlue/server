/**
 * Route matching utilities for FitGlue HTTP handlers.
 * Provides declarative route matching with parameter extraction.
 */

import { FrameworkContext, HttpError } from '../framework';

/**
 * Minimal request interface for routing.
 * Compatible with Express Request without importing it directly.
 */
export interface RoutableRequest {
  method: string;
  path: string;
  query: Record<string, unknown>;
  body?: unknown;
}

/**
 * Result of a successful route match.
 */
export interface RouteMatch {
  /** Path parameters extracted from the route pattern */
  params: Record<string, string>;
  /** Query parameters from the request */
  query: Record<string, string>;
}

/**
 * Route handler definition.
 */
export interface RouteHandler {
  /** HTTP method (GET, POST, PUT, PATCH, DELETE, etc.) */
  method: string;
  /** Route pattern with :param placeholders and * wildcards */
  pattern: string;
  /** Handler function to execute on match */
  handler: (match: RouteMatch, req: RoutableRequest, ctx: FrameworkContext) => Promise<unknown> | unknown;
}

/**
 * Match a request path against a route pattern.
 *
 * Pattern syntax:
 * - `:param` - matches a single path segment, extracts to params.param
 * - `*` - matches any prefix (useful for URL rewriting scenarios)
 * - Literal strings match exactly
 *
 * Examples:
 * - `/api/showcase/:id` matches `/api/showcase/abc123` → {params: {id: 'abc123'}}
 * - `/showcase/:id` matches `/showcase/xyz` → {params: {id: 'xyz'}}
 * - `*pipelines/:id` matches `/api/users/me/pipelines/123` → {params: {id: '123'}}
 * - `*integrations/:provider/connect` matches `/integrations/strava/connect` → {params: {provider: 'strava'}}
 *
 * @param method HTTP method
 * @param path Request path
 * @param pattern Route pattern
 * @returns RouteMatch if matched, null otherwise
 */
export function matchRoute(
  method: string,
  path: string,
  pattern: string
): RouteMatch | null {
  // Normalize paths
  const normalizedPath = path.split('/').filter(Boolean);
  const patternParts = pattern.split('/').filter(Boolean);

  const params: Record<string, string> = {};
  let pathIndex = 0;

  for (let i = 0; i < patternParts.length; i++) {
    const patternPart = patternParts[i];

    // Wildcard matches any prefix
    if (patternPart === '*') {
      // Find the next literal part in pattern
      const nextLiteral = patternParts[i + 1];
      if (!nextLiteral) {
        // Wildcard at end matches rest
        return { params, query: {} };
      }

      // Find where the next literal appears in the path
      const literalIndex = normalizedPath.indexOf(nextLiteral, pathIndex);
      if (literalIndex === -1) {
        return null;
      }
      pathIndex = literalIndex;
      continue;
    }

    // Check if we have enough path segments
    if (pathIndex >= normalizedPath.length) {
      return null;
    }

    // Parameter extraction
    if (patternPart.startsWith(':')) {
      const paramName = patternPart.slice(1);
      params[paramName] = normalizedPath[pathIndex];
      pathIndex++;
      continue;
    }

    // Literal match
    if (patternPart === normalizedPath[pathIndex]) {
      pathIndex++;
      continue;
    }

    // No match
    return null;
  }

  // Pattern must consume all path segments (unless it ends with wildcard)
  if (pathIndex !== normalizedPath.length) {
    return null;
  }

  return { params, query: {} };
}

/**
 * Route a request through a list of handlers.
 * Returns the result of the first matching handler.
 * Throws HttpError(404) if no route matches.
 *
 * @param req Express request object
 * @param ctx Framework context
 * @param routes Array of route handlers
 * @returns Result from the matched handler
 */
export async function routeRequest(
  req: RoutableRequest,
  ctx: FrameworkContext,
  routes: RouteHandler[]
): Promise<unknown> {
  const method = req.method;
  const path = req.path;

  for (const route of routes) {
    // Check method match
    if (route.method !== method) {
      continue;
    }

    // Check pattern match
    const match = matchRoute(method, path, route.pattern);
    if (!match) {
      continue;
    }

    // Add query parameters
    match.query = req.query as Record<string, string>;

    // Execute handler
    return await route.handler(match, req, ctx);
  }

  // No route matched
  throw new HttpError(404, 'Not found');
}
