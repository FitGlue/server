export {
    verifyEmailTemplate,
    passwordResetTemplate,
    welcomeTemplate,
    dataExportTemplate,
    changeEmailTemplate,
    trialExpiringTemplate,
    trialExpiredTemplate,
    registrationSummaryTemplate,
} from './email-templates';

/**
 * Derives the base URL from the GOOGLE_CLOUD_PROJECT environment variable.
 *
 *  - fitglue-server-prod → https://fitglue.tech
 *  - fitglue-server-dev  → https://dev.fitglue.tech
 *  - fitglue-server-test → https://test.fitglue.tech
 */
export function getBaseUrl(): string {
    const project = process.env.GOOGLE_CLOUD_PROJECT || '';
    if (project.endsWith('-dev')) return 'https://dev.fitglue.tech';
    if (project.endsWith('-test')) return 'https://test.fitglue.tech';
    return 'https://fitglue.tech';
}
