import { PluginLifecycleHooks, PluginLifecycleContext } from '../registry';

const GITHUB_API = 'https://api.github.com';

/**
 * Parse owner/repo from a sourceConfig.
 * Expected format: sourceConfig.repo = "owner/repo"
 */
function parseRepo(config: Record<string, string>): { owner: string; repo: string } {
    const repoFull = config.repo || config.repository || '';
    const parts = repoFull.split('/');
    if (parts.length !== 2 || !parts[0] || !parts[1]) {
        throw new Error(`Invalid repo format "${repoFull}". Expected "owner/repo".`);
    }
    return { owner: parts[0], repo: parts[1] };
}

/**
 * GitHub lifecycle hooks — manages webhook registration/deregistration.
 *
 * onPipelineCreate: Registers a push webhook on the configured GitHub repo.
 *   - Throws on failure → blocks pipeline creation.
 *   - Returns { webhook_id } to persist in sourceConfig.
 *
 * onPipelineDelete: Removes the webhook from the repo.
 *   - Best-effort — logged but doesn't block deletion.
 */
export const githubHooks: PluginLifecycleHooks = {
    async onPipelineCreate(ctx: PluginLifecycleContext): Promise<Record<string, string> | void> {
        const { owner, repo } = parseRepo(ctx.config);
        const webhookUrl = process.env.GITHUB_HANDLER_URL;
        const webhookSecret = process.env.GITHUB_WEBHOOK_SECRET;

        if (!webhookUrl) {
            throw new Error('GITHUB_HANDLER_URL environment variable is not set');
        }
        if (!webhookSecret) {
            throw new Error('GITHUB_WEBHOOK_SECRET environment variable is not set');
        }

        const token = await ctx.getValidToken(ctx.userId, 'github');

        ctx.logger.info('Creating GitHub webhook', { owner, repo, pipelineId: ctx.pipelineId });

        const response = await fetch(`${GITHUB_API}/repos/${owner}/${repo}/hooks`, {
            method: 'POST',
            headers: {
                'Authorization': `Bearer ${token}`,
                'Accept': 'application/vnd.github+json',
                'Content-Type': 'application/json',
                'X-GitHub-Api-Version': '2022-11-28',
            },
            body: JSON.stringify({
                name: 'web',
                active: true,
                events: ['push'],
                config: {
                    url: webhookUrl,
                    content_type: 'json',
                    secret: webhookSecret,
                    insecure_ssl: '0',
                },
            }),
        });

        if (!response.ok) {
            const errorBody = await response.text();
            throw new Error(`Failed to create GitHub webhook (${response.status}): ${errorBody}`);
        }

        const webhook = await response.json() as { id: number };
        ctx.logger.info('GitHub webhook created', { owner, repo, webhookId: webhook.id });

        return { webhook_id: String(webhook.id) };
    },

    async onPipelineDelete(ctx: PluginLifecycleContext): Promise<void> {
        const webhookId = ctx.config.webhook_id;
        if (!webhookId) {
            ctx.logger.warn('No webhook_id in sourceConfig, skipping cleanup', { pipelineId: ctx.pipelineId });
            return;
        }

        const { owner, repo } = parseRepo(ctx.config);

        let token: string;
        try {
            token = await ctx.getValidToken(ctx.userId, 'github');
        } catch (err) {
            ctx.logger.warn('Could not get GitHub token for webhook cleanup', { pipelineId: ctx.pipelineId, error: String(err) });
            return;
        }

        ctx.logger.info('Deleting GitHub webhook', { owner, repo, webhookId, pipelineId: ctx.pipelineId });

        try {
            const response = await fetch(`${GITHUB_API}/repos/${owner}/${repo}/hooks/${webhookId}`, {
                method: 'DELETE',
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'Accept': 'application/vnd.github+json',
                    'X-GitHub-Api-Version': '2022-11-28',
                },
            });

            if (response.status === 404) {
                ctx.logger.warn('GitHub webhook already deleted or not found', { owner, repo, webhookId });
            } else if (!response.ok) {
                ctx.logger.warn('Failed to delete GitHub webhook', { owner, repo, webhookId, status: response.status });
            } else {
                ctx.logger.info('GitHub webhook deleted', { owner, repo, webhookId });
            }
        } catch (err) {
            ctx.logger.warn('Error deleting GitHub webhook', { owner, repo, webhookId, error: String(err) });
        }
    },
};
