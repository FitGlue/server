import { BaseConnector, ConnectorConfig, IngestStrategy, FrameworkContext } from '@fitglue/shared/framework';
import { StandardizedActivity, Session, CloudEventSource, ActivitySource, ActivityType } from '@fitglue/shared/types';
import { parseActivityType } from '@fitglue/shared/types';
import { createGitHubClient } from '@fitglue/shared/integrations/github';

// ─── GitHub Webhook Types ───────────────────────────────────────

interface GitHubPushEvent {
    ref: string;
    commits: GitHubCommit[];
    repository: {
        full_name: string;
        owner: { login: string };
        name: string;
    };
    sender: { login: string };
    head_commit?: GitHubCommit;
}

interface GitHubCommit {
    id: string;       // SHA
    message: string;
    timestamp: string;
    added: string[];
    modified: string[];
    removed: string[];
    committer: {
        name: string;
        email: string;
        username?: string;
    };
}

// ─── Connector Config ───────────────────────────────────────────

export interface GitHubConnectorConfig extends ConnectorConfig {
    accessToken: string;
    repo: string;       // e.g. "user/fitness-data"
    folder: string;     // e.g. "workouts/"
}

// ─── Frontmatter Types ──────────────────────────────────────────

interface WorkoutFrontmatter {
    title?: string;
    type?: string;
    fit_file?: string;
    date?: string;
}

// ─── Connector Implementation ───────────────────────────────────

export class GitHubConnector extends BaseConnector<GitHubConnectorConfig, GitHubPushEvent> {
    readonly name = 'github';
    readonly strategy: IngestStrategy = 'webhook';
    readonly cloudEventSource = CloudEventSource.CLOUD_EVENT_SOURCE_GITHUB;
    readonly activitySource = ActivitySource.SOURCE_GITHUB;

    constructor(context: FrameworkContext) {
        super(context);
        context.logger.debug('GitHubConnector: initialized');
    }

    /**
     * Extract the commit SHA as the unique ID from the push event.
     */
    extractId(body: unknown): string | null {
        if (!body) return null;
        const event = body as GitHubPushEvent;

        // Use head_commit SHA if available, otherwise first commit
        return event.head_commit?.id || event.commits?.[0]?.id || null;
    }

    /**
     * Resolve FitGlue userId from the webhook payload.
     * Maps repository.owner.login → GitHub username in user integrations.
     */
    async resolveUser(payload: GitHubPushEvent, context: FrameworkContext): Promise<string | null> {
        const repoOwner = payload.repository?.owner?.login;
        if (!repoOwner) {
            context.logger.warn('GitHubConnector: no repository.owner.login in payload');
            return null;
        }

        // Look up user by GitHub username (set during OAuth)
        const user = await context.services.user.findByGitHubUsername(repoOwner);
        if (!user) {
            context.logger.warn(`GitHubConnector: no user found for GitHub username ${repoOwner}`);
            return null;
        }

        return user.id;
    }

    /**
     * Verify the webhook request. Checks for FitGlue Bot commits
     * to prevent infinite loops (secondary guard beyond protobuf bounceback).
     */
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    async verifyRequest(req: any, _context: FrameworkContext): Promise<{ handled: boolean; response?: any } | undefined> {
        const body = req.body as GitHubPushEvent;

        // Only process push events
        const eventType = req.headers['x-github-event'];
        if (eventType !== 'push') {
            this.context.logger.info(`GitHubConnector: ignoring non-push event: ${eventType}`);
            return { handled: true, response: { status: 200, body: { message: 'Ignored: not a push event' } } };
        }

        // Skip commits by FitGlue Bot (loop prevention)
        const commits = body.commits || [];
        const allBotCommits = commits.every(c =>
            c.committer?.name === 'FitGlue Bot' || c.committer?.email === 'bot@fitglue.com'
        );

        if (allBotCommits && commits.length > 0) {
            this.context.logger.info('GitHubConnector: skipping FitGlue Bot commits (loop prevention)');
            return { handled: true, response: { status: 200, body: { message: 'Ignored: FitGlue Bot commits' } } };
        }

        return undefined; // Continue processing
    }

    /**
     * Fetch and map activities from a GitHub push event.
     * Parses Markdown frontmatter + referenced FIT files from the commit.
     */
    async fetchAndMap(activityId: string, config: GitHubConnectorConfig): Promise<StandardizedActivity[]> {
        this.context.logger.debug(`GitHubConnector: fetching and mapping commit ${activityId}`);
        const activities: StandardizedActivity[] = [];

        const userId = (config as unknown as { userId: string }).userId;
        if (!userId) {
            throw new Error('userId missing in connector config');
        }

        const [owner, repo] = config.repo.split('/');
        if (!owner || !repo) {
            throw new Error(`Invalid repo format: ${config.repo}. Expected 'owner/repo'.`);
        }

        const client = this.createClient(config.accessToken, userId);

        // Fetch the commit details to get changed files
        const { data: commitData, error: commitError } = await client.GET(
            '/repos/{owner}/{repo}/commits/{ref}',
            {
                params: {
                    path: {
                        owner,
                        repo,
                        ref: activityId
                    }
                }
            }
        );

        if (commitError) {
            throw new Error(`Failed to fetch commit ${activityId} from ${config.repo}: ${JSON.stringify(commitError)}`);
        }
        if (!commitData) {
            throw new Error(`Failed to fetch commit ${activityId} from ${config.repo}: No data returned`);
        }

        // Collect all .md files that were added or modified in the configured folder
        const commitFiles = commitData.files ?? [];
        const changedFiles = [
            ...commitFiles.filter(f => f.status === 'added' || f.status === 'modified').map(f => f.filename)
        ].filter(filePath =>
            filePath.startsWith(config.folder) &&
            filePath.endsWith('.md')
        );

        if (changedFiles.length === 0) {
            this.context.logger.info('GitHubConnector: no .md files in configured folder, skipping');
            return [];
        }

        // Process each markdown file
        for (const mdPath of changedFiles) {
            try {
                const activity = await this.processMarkdownFile(
                    mdPath, activityId, config, userId, client
                );
                if (activity) {
                    activities.push(activity);
                }
            } catch (err) {
                this.captureError(err, { file: mdPath, commit: activityId });
                this.context.logger.warn(`GitHubConnector: failed to process ${mdPath}`, { error: err });
            }
        }

        return activities;
    }

    /**
     * Map a raw push event to a StandardizedActivity.
     * This is called by the webhook processor for simple payloads.
     */
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    async mapActivity(rawPayload: GitHubPushEvent, context?: any): Promise<StandardizedActivity> {
        const userId = context?.userId;
        if (!userId) throw new Error('GitHubMapping requires userId in context');

        const commitId = rawPayload.head_commit?.id || rawPayload.commits?.[0]?.id || 'unknown';
        const commitTime = rawPayload.head_commit?.timestamp
            ? new Date(rawPayload.head_commit.timestamp)
            : new Date();

        return {
            source: 'GITHUB',
            externalId: `github_${commitId}`,
            userId,
            startTime: commitTime,
            timeMarkers: [],
            name: rawPayload.head_commit?.message || 'GitHub Activity',
            type: ActivityType.ACTIVITY_TYPE_WORKOUT,
            description: '',
            sessions: [],
            tags: [],
            notes: ''
        };
    }

    // ─── Private Helpers ──────────────────────────────────────────

    /**
     * Process a single Markdown file from the commit.
     * Parses frontmatter, optionally fetches referenced FIT file,
     * and builds a StandardizedActivity.
     */
    private async processMarkdownFile(
        mdPath: string,
        commitSha: string,
        config: GitHubConnectorConfig,
        userId: string,
        client: ReturnType<typeof createGitHubClient>
    ): Promise<StandardizedActivity | null> {
        const [owner, repo] = config.repo.split('/');
        if (!owner || !repo) {
            throw new Error(`Invalid repo format: ${config.repo}. Expected 'owner/repo'.`);
        }

        // Fetch the Markdown file content
        const { data: fileContent, error: fileError } = await client.GET(
            '/repos/{owner}/{repo}/contents/{path}',
            {
                params: {
                    path: {
                        owner,
                        repo,
                        path: mdPath
                    },
                    query: {
                        ref: commitSha
                    }
                }
            }
        );

        if (fileError) {
            this.context.logger.warn(`GitHubConnector: failed to fetch ${mdPath}: ${JSON.stringify(fileError)}`);
            return null;
        }
        if (!fileContent || !('content' in fileContent) || !fileContent.content) {
            this.context.logger.warn(`GitHubConnector: empty file or not a file at ${mdPath}`);
            return null;
        }

        // Decode base64 content
        const decoded = Buffer.from(fileContent.content, 'base64').toString('utf-8');

        // Parse the frontmatter and body
        const { frontmatter, body } = this.parseFrontmatter(decoded);

        // Determine activity metadata
        const title = frontmatter.title || this.generateName(frontmatter, mdPath);
        const activityType = this.mapActivityType(frontmatter.type);
        const activityDate = frontmatter.date
            ? new Date(frontmatter.date)
            : new Date();

        // Build base session
        const session: Session = {
            startTime: activityDate,
            totalElapsedTime: 0,
            totalDistance: 0,
            laps: [],
            strengthSets: []
        };

        // If a FIT file is referenced, fetch and include it as metadata
        const fitFileBase64 = frontmatter.fit_file
            ? await this.fetchFitFileContent(mdPath, frontmatter.fit_file, { owner, repo, commitSha }, client)
            : undefined;

        const activity: StandardizedActivity = {
            source: 'GITHUB',
            externalId: `github_${commitSha}_${mdPath.replace(/[^a-zA-Z0-9]/g, '_')}`,
            userId,
            startTime: activityDate,
            timeMarkers: [],
            name: title,
            type: activityType,
            description: body.trim(),
            sessions: [session],
            tags: [],
            notes: fitFileBase64 ? `fitFileBase64:${fitFileBase64}` : ''
        };

        return activity;
    }

    /**
     * Parse YAML frontmatter from a Markdown string.
     * Expects --- delimiters.
     */
    private parseFrontmatter(content: string): { frontmatter: WorkoutFrontmatter; body: string } {
        const frontmatterRegex = /^---\s*\n([\s\S]*?)\n---\s*\n?([\s\S]*)$/;
        const match = content.match(frontmatterRegex);

        if (!match) {
            return { frontmatter: {}, body: content };
        }

        const yamlStr = match[1];
        const body = match[2];

        // Simple YAML parser for flat key-value pairs
        const frontmatter: WorkoutFrontmatter = {};
        yamlStr.split('\n').forEach(line => {
            const colonIdx = line.indexOf(':');
            if (colonIdx === -1) return;
            const key = line.substring(0, colonIdx).trim();
            const value = line.substring(colonIdx + 1).trim().replace(/^['"]|['"]$/g, '');
            if (key && value) {
                (frontmatter as Record<string, string>)[key] = value;
            }
        });

        return { frontmatter, body };
    }

    /**
     * Fetch a referenced FIT file's base64 content from the repository.
     * Returns undefined if the file cannot be fetched.
     */
    private async fetchFitFileContent(
        mdPath: string,
        fitFile: string,
        repoContext: { owner: string; repo: string; commitSha: string },
        client: ReturnType<typeof createGitHubClient>
    ): Promise<string | undefined> {
        const { owner, repo, commitSha } = repoContext;
        const fitPath = this.resolveFitPath(mdPath, fitFile);
        try {
            const { data: fitContent, error: fitError } = await client.GET(
                '/repos/{owner}/{repo}/contents/{path}',
                {
                    params: {
                        path: {
                            owner,
                            repo,
                            path: fitPath
                        },
                        query: {
                            ref: commitSha
                        }
                    }
                }
            );

            if (fitError) {
                this.context.logger.warn(`GitHubConnector: failed to fetch FIT file at ${fitPath}: ${JSON.stringify(fitError)}`);
                return undefined;
            }

            if (fitContent && ('content' in fitContent) && fitContent.content) {
                return fitContent.content; // Already base64 from GitHub API
            }

            return undefined;
        } catch (err) {
            this.context.logger.warn(`GitHubConnector: failed to fetch FIT file at ${fitPath}`, { error: err });
            return undefined;
        }
    }

    /**
     * Resolve the FIT file path relative to the Markdown file.
     */
    private resolveFitPath(mdPath: string, fitFile: string): string {
        const dir = mdPath.substring(0, mdPath.lastIndexOf('/') + 1);
        return `${dir}${fitFile}`;
    }

    /**
     * Generate a fallback activity name from frontmatter or file path.
     */
    private generateName(frontmatter: WorkoutFrontmatter, filePath: string): string {
        const type = frontmatter.type || 'Activity';
        const date = frontmatter.date || '';
        if (date) {
            return `${type.charAt(0).toUpperCase() + type.slice(1)} — ${date}`;
        }
        // Extract name from file path
        const fileName = filePath.split('/').pop()?.replace('.md', '') || 'Activity';
        return fileName.replace(/[-_]/g, ' ').replace(/\b\w/g, l => l.toUpperCase());
    }

    /**
     * Map user-provided activity type string to ActivityType enum.
     * Uses the generated parseActivityType which handles enum names,
     * display names, and informal aliases (running, cycling, bike, etc.).
     */
    private mapActivityType(type?: string): ActivityType {
        if (!type) return ActivityType.ACTIVITY_TYPE_WORKOUT;
        const parsed = parseActivityType(type);

        // Treat UNSPECIFIED as OTHER for source connectors
        return parsed === ActivityType.ACTIVITY_TYPE_UNSPECIFIED
            ? ActivityType.ACTIVITY_TYPE_WORKOUT
            : parsed;
    }

    /**
   * Create a typed GitHub API client using the user's OAuth token.
   */
    private createClient(accessToken: string, userId?: string) {
        return createGitHubClient({
            accessToken,
            usageTracking: userId ? {
                userStore: this.context.stores.users,
                userId
            } : undefined,
            logger: this.context.logger
        });
    }
}

// Re-export for use by other modules
export type { GitHubPushEvent };
