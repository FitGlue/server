/**
 * Tests for the GitHubConnector.
 *
 * All @fitglue/shared/* modules are mocked so the test suite doesn't depend
 * on internal shared library implementation details. Expected enum values
 * are asserted by string / number literals that match the mock definitions.
 */

// ── Enum constants (mirror the mock factory values below) ───────
const ACTIVITY_TYPE_WORKOUT = 100;
const ACTIVITY_TYPE_RUN = 1;
const CLOUD_EVENT_SOURCE_GITHUB = 'CLOUD_EVENT_SOURCE_GITHUB';
const ACTIVITY_SOURCE_GITHUB = 'SOURCE_GITHUB';

// ── Mock shared modules ─────────────────────────────────────────

jest.mock('@fitglue/shared/types', () => ({
    ActivityType: {
        ACTIVITY_TYPE_UNSPECIFIED: 0,
        ACTIVITY_TYPE_RUN: 1,
        ACTIVITY_TYPE_RIDE: 2,
        ACTIVITY_TYPE_WORKOUT: 100,
    },
    CloudEventSource: {
        CLOUD_EVENT_SOURCE_GITHUB: 'CLOUD_EVENT_SOURCE_GITHUB',
    },
    ActivitySource: {
        SOURCE_GITHUB: 'SOURCE_GITHUB',
    },
    parseActivityType: (type: string) => {
        const map: Record<string, number> = { running: 1, cycling: 2 };
        return map[type.toLowerCase()] ?? 0;
    },
}));

jest.mock('@fitglue/shared/integrations/github', () => ({
    createGitHubClient: jest.fn(() => ({ GET: jest.fn() })),
}));

jest.mock('@fitglue/shared/framework', () => ({
    BaseConnector: class {
        protected _context: unknown;
        constructor(context: unknown) { this._context = context; }
        protected get context() { return this._context; }
        // eslint-disable-next-line @typescript-eslint/no-empty-function
        protected captureError() { }
    },
}));

import { GitHubConnector } from './connector';

// ── Helpers ─────────────────────────────────────────────────────

function createMockContext(overrides: Record<string, unknown> = {}) {
    return {
        logger: {
            info: jest.fn(),
            warn: jest.fn(),
            error: jest.fn(),
            debug: jest.fn(),
        },
        stores: {
            users: {
                findByField: jest.fn(),
                updateLastUsed: jest.fn(),
            },
        },
        services: {
            user: { findByGitHubUsername: jest.fn() },
        },
        ...overrides,
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any;
}

function createPushEvent(overrides: Record<string, unknown> = {}) {
    return {
        ref: 'refs/heads/main',
        commits: [
            {
                id: 'abc123',
                message: 'Add morning run',
                timestamp: '2026-02-09T08:00:00Z',
                added: ['workouts/morning-run.md'],
                modified: [],
                removed: [],
                committer: { name: 'Jane', email: 'jane@example.com', username: 'jane' },
            },
        ],
        repository: {
            full_name: 'jane/fitness-data',
            owner: { login: 'jane' },
            name: 'fitness-data',
        },
        sender: { login: 'jane' },
        head_commit: {
            id: 'abc123',
            message: 'Add morning run',
            timestamp: '2026-02-09T08:00:00Z',
            added: ['workouts/morning-run.md'],
            modified: [],
            removed: [],
            committer: { name: 'Jane', email: 'jane@example.com', username: 'jane' },
        },
        ...overrides,
    };
}

// ── Tests ───────────────────────────────────────────────────────

describe('GitHubConnector', () => {
    let connector: GitHubConnector;
    let ctx: ReturnType<typeof createMockContext>;

    beforeEach(() => {
        jest.clearAllMocks();
        ctx = createMockContext();
        connector = new GitHubConnector(ctx);
    });

    // ── Static properties ───────────────────────────────────

    it('should have correct connector metadata', () => {
        expect(connector.name).toBe('github');
        expect(connector.strategy).toBe('webhook');
        expect(connector.cloudEventSource).toBe(CLOUD_EVENT_SOURCE_GITHUB);
        expect(connector.activitySource).toBe(ACTIVITY_SOURCE_GITHUB);
    });

    // ── extractId ───────────────────────────────────────────

    describe('extractId', () => {
        it('should return head_commit SHA when available', () => {
            expect(connector.extractId(createPushEvent())).toBe('abc123');
        });

        it('should fall back to first commit SHA', () => {
            expect(connector.extractId(createPushEvent({ head_commit: undefined }))).toBe('abc123');
        });

        it('should return null for empty payload', () => {
            expect(connector.extractId(null)).toBeNull();
        });

        it('should return null when no commits', () => {
            expect(connector.extractId(createPushEvent({ head_commit: undefined, commits: [] }))).toBeNull();
        });
    });

    // ── resolveUser ─────────────────────────────────────────

    describe('resolveUser', () => {
        it('should resolve user by GitHub username', async () => {
            ctx.services.user.findByGitHubUsername.mockResolvedValue({ id: 'user-123', data: {} });
            const result = await connector.resolveUser(createPushEvent() as any, ctx);
            expect(result).toBe('user-123');
            expect(ctx.services.user.findByGitHubUsername).toHaveBeenCalledWith('jane');
        });

        it('should return null when no user found', async () => {
            ctx.services.user.findByGitHubUsername.mockResolvedValue(null);
            const result = await connector.resolveUser(createPushEvent() as any, ctx);
            expect(result).toBeNull();
            expect(ctx.logger.warn).toHaveBeenCalledWith(expect.stringContaining('no user found'));
        });

        it('should return null when owner login is missing', async () => {
            const event = createPushEvent({
                repository: { full_name: 'jane/repo', name: 'repo' },
            });
            const result = await connector.resolveUser(event as any, ctx);
            expect(result).toBeNull();
            expect(ctx.logger.warn).toHaveBeenCalledWith(expect.stringContaining('no repository.owner.login'));
        });
    });

    // ── verifyRequest ───────────────────────────────────────

    describe('verifyRequest', () => {
        it('should ignore non-push events', async () => {
            const req = { headers: { 'x-github-event': 'ping' }, body: createPushEvent() };
            const result = await connector.verifyRequest(req, ctx);
            expect(result).toEqual({
                handled: true,
                response: { status: 200, body: { message: 'Ignored: not a push event' } },
            });
        });

        it('should skip FitGlue Bot commits (loop prevention)', async () => {
            const req = {
                headers: { 'x-github-event': 'push' },
                body: createPushEvent({
                    commits: [{
                        id: 'bot-1', message: 'Auto-sync', timestamp: '2026-02-09T08:00:00Z',
                        added: [], modified: ['data.md'], removed: [],
                        committer: { name: 'FitGlue Bot', email: 'bot@fitglue.com' },
                    }],
                }),
            };
            const result = await connector.verifyRequest(req, ctx);
            expect(result).toEqual({
                handled: true,
                response: { status: 200, body: { message: 'Ignored: FitGlue Bot commits' } },
            });
        });

        it('should skip by email match', async () => {
            const req = {
                headers: { 'x-github-event': 'push' },
                body: createPushEvent({
                    commits: [{
                        id: 'bot-2', message: 'Update', timestamp: '2026-02-09T08:00:00Z',
                        added: [], modified: [], removed: [],
                        committer: { name: 'SomeBot', email: 'bot@fitglue.com' },
                    }],
                }),
            };
            const result = await connector.verifyRequest(req, ctx);
            expect(result?.handled).toBe(true);
        });

        it('should allow normal push events through', async () => {
            const req = { headers: { 'x-github-event': 'push' }, body: createPushEvent() };
            const result = await connector.verifyRequest(req, ctx);
            expect(result).toBeUndefined();
        });

        it('should allow mixed bot and user commits through', async () => {
            const req = {
                headers: { 'x-github-event': 'push' },
                body: createPushEvent({
                    commits: [
                        {
                            id: 'bot-1', message: 'Bot', timestamp: '2026-02-09T08:00:00Z',
                            added: [], modified: [], removed: [],
                            committer: { name: 'FitGlue Bot', email: 'bot@fitglue.com' },
                        },
                        {
                            id: 'user-1', message: 'User', timestamp: '2026-02-09T08:01:00Z',
                            added: [], modified: [], removed: [],
                            committer: { name: 'Jane', email: 'jane@example.com' },
                        },
                    ],
                }),
            };
            const result = await connector.verifyRequest(req, ctx);
            expect(result).toBeUndefined();
        });
    });

    // ── mapActivity ─────────────────────────────────────────

    describe('mapActivity', () => {
        it('should map push event to StandardizedActivity', async () => {
            const activity = await connector.mapActivity(createPushEvent() as any, { userId: 'u1' });
            expect(activity.source).toBe('GITHUB');
            expect(activity.externalId).toBe('github_abc123');
            expect(activity.userId).toBe('u1');
            expect(activity.name).toBe('Add morning run');
            expect(activity.type).toBe(ACTIVITY_TYPE_WORKOUT);
            expect(activity.startTime).toEqual(new Date('2026-02-09T08:00:00Z'));
        });

        it('should throw without userId in context', async () => {
            await expect(connector.mapActivity(createPushEvent() as any, {})).rejects.toThrow('userId');
        });

        it('should use fallback name when no head_commit message', async () => {
            const event = createPushEvent({
                head_commit: { id: 'sha1', timestamp: '2026-02-09T08:00:00Z' },
            });
            const activity = await connector.mapActivity(event as any, { userId: 'u1' });
            expect(activity.name).toBe('GitHub Activity');
        });
    });

    // ── Private helpers (bracket notation) ──────────────────

    describe('parseFrontmatter', () => {
        const parse = (content: string) => (connector as any).parseFrontmatter(content);

        it('should parse valid frontmatter', () => {
            const md = '---\ntitle: Morning Run\ntype: running\ndate: 2026-02-09\n---\nFelt great today!';
            const { frontmatter, body } = parse(md);
            expect(frontmatter.title).toBe('Morning Run');
            expect(frontmatter.type).toBe('running');
            expect(frontmatter.date).toBe('2026-02-09');
            expect(body).toBe('Felt great today!');
        });

        it('should handle missing frontmatter', () => {
            const md = 'Just a regular markdown file.';
            const { frontmatter, body } = parse(md);
            expect(frontmatter).toEqual({});
            expect(body).toBe(md);
        });

        it('should strip quotes from values', () => {
            const md = '---\ntitle: "Quoted Title"\n---\nBody';
            const { frontmatter } = parse(md);
            expect(frontmatter.title).toBe('Quoted Title');
        });

        it('should handle frontmatter with fit_file reference', () => {
            const md = '---\ntitle: Track Session\nfit_file: session.fit\n---\nIntervals workout';
            const { frontmatter, body } = parse(md);
            expect(frontmatter.fit_file).toBe('session.fit');
            expect(body).toBe('Intervals workout');
        });
    });

    describe('mapActivityType', () => {
        const mapType = (type?: string) => (connector as any).mapActivityType(type);

        it('should return WORKOUT for undefined', () => {
            expect(mapType(undefined)).toBe(ACTIVITY_TYPE_WORKOUT);
        });

        it('should return WORKOUT for empty string', () => {
            expect(mapType('')).toBe(ACTIVITY_TYPE_WORKOUT);
        });

        it('should parse known activity types', () => {
            expect(mapType('running')).toBe(ACTIVITY_TYPE_RUN);
        });

        it('should fallback to WORKOUT for unrecognized types', () => {
            expect(mapType('underwater_basket_weaving')).toBe(ACTIVITY_TYPE_WORKOUT);
        });
    });

    describe('generateName', () => {
        const genName = (fm: Record<string, string>, filePath: string) =>
            (connector as any).generateName(fm, filePath);

        it('should use type and date when both available', () => {
            expect(genName({ type: 'running', date: '2026-02-09' }, 'workouts/run.md'))
                .toBe('Running — 2026-02-09');
        });

        it('should fallback to file path when no date', () => {
            expect(genName({ type: 'cycling' }, 'workouts/morning-ride.md'))
                .toBe('Morning Ride');
        });

        it('should use Activity as fallback type with date', () => {
            expect(genName({ date: '2026-02-09' }, 'workouts/run.md'))
                .toBe('Activity — 2026-02-09');
        });

        it('should handle hyphens and underscores in file names', () => {
            expect(genName({}, 'workouts/long_tempo-run.md'))
                .toBe('Long Tempo Run');
        });
    });

    describe('resolveFitPath', () => {
        const resolve = (mdPath: string, fitFile: string) =>
            (connector as any).resolveFitPath(mdPath, fitFile);

        it('should resolve FIT file relative to markdown directory', () => {
            expect(resolve('workouts/2026/morning-run.md', 'session.fit'))
                .toBe('workouts/2026/session.fit');
        });

        it('should handle root-level markdown files', () => {
            expect(resolve('run.md', 'data.fit')).toBe('data.fit');
        });
    });
});
