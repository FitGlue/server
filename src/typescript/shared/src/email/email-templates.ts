/**
 * FitGlue Email Template Library
 *
 * Branded HTML email templates using the FitGlue design system.
 * All templates use inline styles and table-based layout for
 * maximum email client compatibility.
 *
 * Brand tokens:
 *  - Primary:   #FF1B8D (pink)
 *  - Secondary: #9D4EDD (purple)
 *  - Accent:    #4CC9F0 (cyan)
 *  - Background:#0A0A0A (near-black)
 */

// ─── Brand Constants ────────────────────────────────────────────────

const BRAND = {
  primary: '#FF1B8D',
  secondary: '#9D4EDD',
  accent: '#4CC9F0',
  bgDark: '#0A0A0A',
  bgBody: '#f4f4f7',
  bgCard: '#ffffff',
  textPrimary: '#1a1a2e',
  textSecondary: '#555770',
  textMuted: '#8e8ea0',
  border: '#e4e4e7',
  footerBg: '#fafafa',
} as const;



// ─── Shared Layout ──────────────────────────────────────────────────

interface EmailLayoutOptions {
  /** The inner content HTML. */
  content: string;
  /** Preview text shown in email list (before opening). */
  previewText: string;
  /** Base URL for the current environment (e.g. https://fitglue.tech). */
  baseUrl: string;
}

/**
 * Wraps email content in the branded FitGlue layout.
 *
 * Structure:
 *  - Dark header with FitGlue wordmark
 *  - White card body
 *  - Light footer with links
 */
function renderLayout({ content, previewText, baseUrl }: EmailLayoutOptions): string {
  return `<!DOCTYPE html>
<html lang="en" xmlns="http://www.w3.org/1999/xhtml">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta http-equiv="X-UA-Compatible" content="IE=edge">
  <title>FitGlue</title>
  <!--[if mso]>
  <noscript><xml><o:OfficeDocumentSettings><o:PixelsPerInch>96</o:PixelsPerInch></o:OfficeDocumentSettings></xml></noscript>
  <![endif]-->
</head>
<body style="margin:0;padding:0;background-color:${BRAND.bgBody};font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Oxygen,Ubuntu,Cantarell,'Helvetica Neue',Arial,sans-serif;-webkit-font-smoothing:antialiased;">
  <!-- Preview text (hidden) -->
  <div style="display:none;max-height:0;overflow:hidden;mso-hide:all;">${previewText}</div>
  <div style="display:none;max-height:0;overflow:hidden;mso-hide:all;">&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;</div>

  <!-- Outer wrapper -->
  <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="background-color:${BRAND.bgBody};">
    <tr>
      <td align="center" style="padding:40px 16px;">
        <!-- Inner card -->
        <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="600" style="max-width:600px;width:100%;">

          <!-- HEADER -->
          <tr>
            <td style="background:${BRAND.bgDark};padding:32px 40px;border-radius:16px 16px 0 0;text-align:center;">
              <a href="${baseUrl}" style="text-decoration:none;">
                <span style="font-size:32px;font-weight:900;letter-spacing:-0.02em;">
                  <span style="color:${BRAND.primary};">Fit</span><span style="color:${BRAND.secondary};">Glue</span>
                </span>
              </a>
            </td>
          </tr>

          <!-- BODY -->
          <tr>
            <td style="background:${BRAND.bgCard};padding:40px;border-left:1px solid ${BRAND.border};border-right:1px solid ${BRAND.border};">
              ${content}
            </td>
          </tr>

          <!-- FOOTER -->
          <tr>
            <td style="background:${BRAND.footerBg};padding:24px 40px;border-radius:0 0 16px 16px;border:1px solid ${BRAND.border};border-top:none;">
              <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%">
                <tr>
                  <td style="text-align:center;padding-bottom:12px;">
                    <a href="${baseUrl}" style="color:${BRAND.textMuted};font-size:12px;text-decoration:none;margin:0 8px;">Website</a>
                    <span style="color:${BRAND.border};">•</span>
                    <a href="https://discord.gg/fitglue" style="color:${BRAND.textMuted};font-size:12px;text-decoration:none;margin:0 8px;">Community</a>
                    <span style="color:${BRAND.border};">•</span>
                    <a href="mailto:support@fitglue.tech" style="color:${BRAND.textMuted};font-size:12px;text-decoration:none;margin:0 8px;">Support</a>
                  </td>
                </tr>
                <tr>
                  <td style="text-align:center;">
                    <p style="color:${BRAND.textMuted};font-size:11px;margin:0;line-height:1.5;">
                      © ${new Date().getFullYear()} FitGlue. Your fitness data, your way.
                    </p>
                  </td>
                </tr>
              </table>
            </td>
          </tr>

        </table>
      </td>
    </tr>
  </table>
</body>
</html>`;
}

// ─── Shared Components ──────────────────────────────────────────────

function ctaButton(text: string, url: string): string {
  return `<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="margin:32px 0;">
  <tr>
    <td align="center">
      <a href="${url}" target="_blank" style="display:inline-block;padding:16px 40px;background:linear-gradient(135deg,${BRAND.primary},${BRAND.secondary});color:#ffffff;text-decoration:none;border-radius:10px;font-weight:700;font-size:16px;letter-spacing:0.02em;mso-padding-alt:16px 40px;">
        ${text}
      </a>
    </td>
  </tr>
</table>`;
}

function heading(text: string): string {
  return `<h1 style="color:${BRAND.textPrimary};font-size:24px;font-weight:700;margin:0 0 16px;line-height:1.3;">${text}</h1>`;
}

function paragraph(text: string): string {
  return `<p style="color:${BRAND.textSecondary};font-size:16px;line-height:1.6;margin:0 0 16px;">${text}</p>`;
}

function smallText(text: string): string {
  return `<p style="color:${BRAND.textMuted};font-size:13px;line-height:1.5;margin:16px 0 0;">${text}</p>`;
}

function divider(): string {
  return `<hr style="border:none;border-top:1px solid ${BRAND.border};margin:24px 0;">`;
}

function emoji(char: string): string {
  return `<span style="font-size:48px;display:block;text-align:center;margin-bottom:16px;">${char}</span>`;
}

// ─── Template Functions ─────────────────────────────────────────────

/**
 * Email #1: Verify Email
 * Sent after registration with email/password.
 */
export function verifyEmailTemplate(verificationUrl: string, baseUrl: string): string {
  return renderLayout({
    baseUrl,
    previewText: 'Verify your email to start using FitGlue',
    content: [
      emoji('✉️'),
      heading('Verify your email address'),
      paragraph('Thanks for signing up for FitGlue! Please confirm your email address by clicking the button below.'),
      ctaButton('Verify Email', verificationUrl),
      paragraph('This link will expire in 24 hours. If you didn\'t create a FitGlue account, you can safely ignore this email.'),
      divider(),
      smallText(`If the button doesn't work, copy and paste this URL into your browser: <a href="${verificationUrl}" style="color:${BRAND.primary};word-break:break-all;">${verificationUrl}</a>`),
    ].join('\n'),
  });
}

/**
 * Email #2: Password Reset
 * Sent when a user requests "Forgot Password".
 */
export function passwordResetTemplate(resetUrl: string, baseUrl: string): string {
  return renderLayout({
    baseUrl,
    previewText: 'Reset your FitGlue password',
    content: [
      emoji('🔐'),
      heading('Reset your password'),
      paragraph('We received a request to reset your FitGlue password. Click the button below to choose a new password.'),
      ctaButton('Reset Password', resetUrl),
      paragraph('This link will expire in 1 hour. If you didn\'t request a password reset, you can safely ignore this email — your password won\'t be changed.'),
      divider(),
      smallText(`If the button doesn't work, copy and paste this URL into your browser: <a href="${resetUrl}" style="color:${BRAND.primary};word-break:break-all;">${resetUrl}</a>`),
    ].join('\n'),
  });
}

/**
 * Email #3: Welcome
 * Sent after email verification is completed.
 */
export function welcomeTemplate(baseUrl: string): string {
  const dashboardUrl = `${baseUrl}/app`;
  return renderLayout({
    baseUrl,
    previewText: 'Welcome to FitGlue — your fitness data, your way',
    content: [
      emoji('🎉'),
      heading('Welcome to FitGlue!'),
      paragraph('Your email has been verified and your account is all set. You\'re ready to start connecting your fitness services and building powerful pipelines.'),
      `<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="margin:24px 0;">
  <tr>
    <td style="background:${BRAND.bgBody};border-radius:10px;padding:20px 24px;">
      <p style="color:${BRAND.textPrimary};font-size:15px;font-weight:600;margin:0 0 12px;">Here's what you can do next:</p>
      <table role="presentation" cellpadding="0" cellspacing="0" border="0">
        <tr><td style="padding:4px 0;color:${BRAND.textSecondary};font-size:14px;">🔗 Connect Strava, Fitbit, Hevy, and more</td></tr>
        <tr><td style="padding:4px 0;color:${BRAND.textSecondary};font-size:14px;">⚡ Set up automated pipelines with boosters</td></tr>
        <tr><td style="padding:4px 0;color:${BRAND.textSecondary};font-size:14px;">🏆 Share your showcase profile with friends</td></tr>
      </table>
    </td>
  </tr>
</table>`,
      paragraph('You have a <strong style="color:' + BRAND.textPrimary + ';">30-day free trial</strong> of our Athlete tier, giving you full access to all features.'),
      ctaButton('Go to Dashboard', dashboardUrl),
    ].join('\n'),
  });
}

/**
 * Email #4: Data Export Ready
 * Sent when the user's data export ZIP is uploaded.
 */
export function dataExportTemplate(downloadUrl: string, baseUrl: string): string {
  return renderLayout({
    baseUrl,
    previewText: 'Your FitGlue data export is ready to download',
    content: [
      emoji('📦'),
      heading('Your data export is ready'),
      paragraph('Your FitGlue data export has been prepared and is ready to download. The file contains all of your account data in JSON format.'),
      ctaButton('Download My Data', downloadUrl),
      paragraph('This download link will expire in <strong style="color:' + BRAND.textPrimary + ';">24 hours</strong>. If you didn\'t request this export, please contact us at <a href="mailto:support@fitglue.tech" style="color:' + BRAND.primary + ';">support@fitglue.tech</a>.'),
    ].join('\n'),
  });
}

/**
 * Email #5: Change Email Confirmation
 * Sent when a user requests to change their email address.
 */
export function changeEmailTemplate(verificationUrl: string, newEmail: string, baseUrl: string): string {
  return renderLayout({
    baseUrl,
    previewText: 'Confirm your new FitGlue email address',
    content: [
      emoji('📧'),
      heading('Confirm your new email'),
      paragraph(`You requested to change your FitGlue email address to <strong style="color:${BRAND.textPrimary};">${newEmail}</strong>. Please confirm this change by clicking the button below.`),
      ctaButton('Confirm Email Change', verificationUrl),
      paragraph('If you didn\'t request this change, please ignore this email and your account will remain unchanged. You may also want to update your password for security.'),
      divider(),
      smallText(`If the button doesn't work, copy and paste this URL into your browser: <a href="${verificationUrl}" style="color:${BRAND.primary};word-break:break-all;">${verificationUrl}</a>`),
    ].join('\n'),
  });
}

/**
 * Email #6: Trial Expiring Soon
 * Sent 3 days before the trial ends.
 */
export function trialExpiringTemplate(daysLeft: number, baseUrl: string): string {
  const upgradeUrl = `${baseUrl}/app/subscription`;
  return renderLayout({
    baseUrl,
    previewText: `Your FitGlue Athlete trial ends in ${daysLeft} day${daysLeft === 1 ? '' : 's'}`,
    content: [
      emoji('⏳'),
      heading(`Your trial ends in ${daysLeft} day${daysLeft === 1 ? '' : 's'}`),
      paragraph('Your free Athlete trial is coming to an end. After it expires, your account will switch to our Hobbyist tier, and some features will be limited.'),
      `<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="margin:24px 0;">
  <tr>
    <td style="background:${BRAND.bgBody};border-radius:10px;padding:20px 24px;">
      <p style="color:${BRAND.textPrimary};font-size:15px;font-weight:600;margin:0 0 12px;">What you'll keep on Hobbyist:</p>
      <table role="presentation" cellpadding="0" cellspacing="0" border="0">
        <tr><td style="padding:4px 0;color:${BRAND.textSecondary};font-size:14px;">✅ 1 active pipeline</td></tr>
        <tr><td style="padding:4px 0;color:${BRAND.textSecondary};font-size:14px;">✅ Basic boosters</td></tr>
      </table>
      <p style="color:${BRAND.textPrimary};font-size:15px;font-weight:600;margin:16px 0 12px;">What requires Athlete:</p>
      <table role="presentation" cellpadding="0" cellspacing="0" border="0">
        <tr><td style="padding:4px 0;color:${BRAND.textSecondary};font-size:14px;">🔒 Unlimited pipelines</td></tr>
        <tr><td style="padding:4px 0;color:${BRAND.textSecondary};font-size:14px;">🔒 Premium boosters &amp; integrations</td></tr>
        <tr><td style="padding:4px 0;color:${BRAND.textSecondary};font-size:14px;">🔒 Showcase profile</td></tr>
      </table>
    </td>
  </tr>
</table>`,
      ctaButton('Upgrade to Athlete', upgradeUrl),
    ].join('\n'),
  });
}

/**
 * Email #7: Trial Expired
 * Sent after the trial has ended.
 */
export function trialExpiredTemplate(baseUrl: string): string {
  const upgradeUrl = `${baseUrl}/app/subscription`;
  return renderLayout({
    baseUrl,
    previewText: 'Your FitGlue Athlete trial has ended',
    content: [
      emoji('⌛'),
      heading('Your Athlete trial has ended'),
      paragraph('Your 30-day Athlete trial has expired and your account has been moved to our free Hobbyist tier. Your data is safe — nothing has been deleted.'),
      paragraph('Upgrade anytime to unlock all Athlete features again, including unlimited pipelines, premium boosters, and your showcase profile.'),
      ctaButton('Upgrade to Athlete', upgradeUrl),
      divider(),
      smallText('If you have any questions about plans or pricing, reach out to us at <a href="mailto:support@fitglue.tech" style="color:' + BRAND.primary + ';">support@fitglue.tech</a>.'),
    ].join('\n'),
  });
}

/**
 * Admin email: Registration Summary
 * Daily digest of new signups for admin.
 */
export function registrationSummaryTemplate(
  dateStr: string,
  users: Array<{ email: string; accessEnabled: boolean; createdAt: Date }>,
  baseUrl: string,
): string {
  if (users.length === 0) {
    return renderLayout({
      baseUrl,
      previewText: `No new registrations — ${dateStr}`,
      content: [
        emoji('📊'),
        heading('Daily Registration Summary'),
        paragraph(`<strong style="color:${BRAND.textPrimary};">Date:</strong> ${dateStr}`),
        paragraph('No new registrations in the last 24 hours.'),
      ].join('\n'),
    });
  }

  const waitingCount = users.filter(u => !u.accessEnabled).length;
  const enabledCount = users.filter(u => u.accessEnabled).length;

  const userRows = users
    .slice(0, 50)
    .map(u => `<tr>
  <td style="padding:10px 12px;border-bottom:1px solid ${BRAND.border};color:${BRAND.textPrimary};font-size:14px;">${u.email}</td>
  <td style="padding:10px 12px;border-bottom:1px solid ${BRAND.border};font-size:14px;">${u.accessEnabled
        ? `<span style="color:#10b981;font-weight:600;">✓ Enabled</span>`
        : `<span style="color:#f59e0b;font-weight:600;">⏳ Waiting</span>`
      }</td>
  <td style="padding:10px 12px;border-bottom:1px solid ${BRAND.border};color:${BRAND.textMuted};font-size:13px;">${u.createdAt.toISOString().split('T')[0]}</td>
</tr>`)
    .join('');

  const moreNote = users.length > 50
    ? `<p style="color:${BRAND.textMuted};font-size:13px;margin-top:8px;">... and ${users.length - 50} more</p>`
    : '';

  return renderLayout({
    baseUrl,
    previewText: `${users.length} new registration${users.length === 1 ? '' : 's'} — ${dateStr}`,
    content: [
      emoji('📊'),
      heading('Daily Registration Summary'),
      paragraph(`<strong style="color:${BRAND.textPrimary};">Date:</strong> ${dateStr}`),
      // Stats cards
      `<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="margin:24px 0;">
  <tr>
    <td style="background:${BRAND.bgBody};border-radius:10px;padding:20px 24px;width:33%;text-align:center;">
      <p style="color:${BRAND.primary};font-size:28px;font-weight:800;margin:0;">${users.length}</p>
      <p style="color:${BRAND.textMuted};font-size:12px;margin:4px 0 0;text-transform:uppercase;letter-spacing:0.05em;">Total</p>
    </td>
    <td style="width:8px;"></td>
    <td style="background:${BRAND.bgBody};border-radius:10px;padding:20px 24px;width:33%;text-align:center;">
      <p style="color:#f59e0b;font-size:28px;font-weight:800;margin:0;">${waitingCount}</p>
      <p style="color:${BRAND.textMuted};font-size:12px;margin:4px 0 0;text-transform:uppercase;letter-spacing:0.05em;">Waiting</p>
    </td>
    <td style="width:8px;"></td>
    <td style="background:${BRAND.bgBody};border-radius:10px;padding:20px 24px;width:33%;text-align:center;">
      <p style="color:#10b981;font-size:28px;font-weight:800;margin:0;">${enabledCount}</p>
      <p style="color:${BRAND.textMuted};font-size:12px;margin:4px 0 0;text-transform:uppercase;letter-spacing:0.05em;">Enabled</p>
    </td>
  </tr>
</table>`,
      // Users table
      `<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="border:1px solid ${BRAND.border};border-radius:8px;overflow:hidden;">
  <tr style="background:${BRAND.bgBody};">
    <th style="padding:10px 12px;text-align:left;font-size:12px;color:${BRAND.textMuted};text-transform:uppercase;letter-spacing:0.05em;font-weight:600;">Email</th>
    <th style="padding:10px 12px;text-align:left;font-size:12px;color:${BRAND.textMuted};text-transform:uppercase;letter-spacing:0.05em;font-weight:600;">Status</th>
    <th style="padding:10px 12px;text-align:left;font-size:12px;color:${BRAND.textMuted};text-transform:uppercase;letter-spacing:0.05em;font-weight:600;">Registered</th>
  </tr>
  ${userRows}
</table>`,
      moreNote,
    ].join('\n'),
  });
}
