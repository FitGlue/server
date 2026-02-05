/**
 * Registration Summary Handler
 *
 * Triggered daily via Cloud Scheduler -> Pub/Sub to send a summary email
 * of new user registrations to james@fitglue.tech.
 */

import { initializeApp } from 'firebase-admin/app';
import { getFirestore } from 'firebase-admin/firestore';
import { getAuth } from 'firebase-admin/auth';
import { SecretManagerServiceClient } from '@google-cloud/secret-manager';
import * as nodemailer from 'nodemailer';
import { UserStore } from '@fitglue/shared/storage';
import { CloudEvent } from '@google-cloud/functions-framework';

// Initialize Firebase Admin
initializeApp();
const db = getFirestore();
const auth = getAuth();
const userStore = new UserStore(db);
const secretClient = new SecretManagerServiceClient();

// Configuration
const RECIPIENT_EMAIL = 'james@fitglue.tech';
const SENDER_EMAIL = 'system@fitglue.tech';
const PROJECT_ID = process.env.GOOGLE_CLOUD_PROJECT || '';

interface UserSummary {
    email: string;
    userId: string;
    createdAt: Date;
    accessEnabled: boolean;
}

/**
 * Fetch the email app password from Secret Manager.
 */
async function getEmailPassword(): Promise<string> {
    const secretName = `projects/${PROJECT_ID}/secrets/email-app-password/versions/latest`;
    const [version] = await secretClient.accessSecretVersion({ name: secretName });
    const payload = version.payload?.data;
    if (!payload) {
        throw new Error('Email app password secret has no payload');
    }
    return typeof payload === 'string' ? payload : Buffer.from(payload).toString('utf8');
}

/**
 * Build the email HTML content.
 */
function buildEmailContent(users: UserSummary[], dateStr: string): string {
    if (users.length === 0) {
        return `
      <h2>📊 FitGlue Daily Registration Summary</h2>
      <p><strong>Date:</strong> ${dateStr}</p>
      <p>No new registrations in the last 24 hours.</p>
      <hr>
      <p style="color: #666; font-size: 12px;">Sent from FitGlue Server</p>
    `;
    }

    const waitingForAccess = users.filter(u => !u.accessEnabled).length;
    const accessEnabled = users.filter(u => u.accessEnabled).length;

    const userRows = users
        .slice(0, 50) // Cap at 50 to avoid huge emails
        .map(u => `
      <tr>
        <td style="padding: 8px; border-bottom: 1px solid #eee;">${u.email}</td>
        <td style="padding: 8px; border-bottom: 1px solid #eee;">
          ${u.accessEnabled
                ? '<span style="color: green;">✓ Enabled</span>'
                : '<span style="color: orange;">⏳ Waiting</span>'}
        </td>
        <td style="padding: 8px; border-bottom: 1px solid #eee; color: #666; font-size: 12px;">
          ${u.createdAt.toISOString().split('T')[0]}
        </td>
      </tr>
    `)
        .join('');

    const moreNote = users.length > 50
        ? `<p style="color: #666;">... and ${users.length - 50} more</p>`
        : '';

    return `
    <h2>📊 FitGlue Daily Registration Summary</h2>
    <p><strong>Date:</strong> ${dateStr}</p>
    
    <div style="background: #f5f5f5; padding: 16px; border-radius: 8px; margin: 16px 0;">
      <h3 style="margin: 0 0 8px 0;">📈 Summary</h3>
      <p style="margin: 4px 0;"><strong>Total new registrations:</strong> ${users.length}</p>
      <p style="margin: 4px 0;"><strong>Waiting for access:</strong> ${waitingForAccess}</p>
      <p style="margin: 4px 0;"><strong>Access enabled:</strong> ${accessEnabled}</p>
    </div>
    
    <h3>New Users</h3>
    <table style="width: 100%; border-collapse: collapse;">
      <thead>
        <tr style="background: #f0f0f0;">
          <th style="padding: 8px; text-align: left;">Email</th>
          <th style="padding: 8px; text-align: left;">Status</th>
          <th style="padding: 8px; text-align: left;">Registered</th>
        </tr>
      </thead>
      <tbody>
        ${userRows}
      </tbody>
    </table>
    ${moreNote}
    
    <hr style="margin-top: 24px;">
    <p style="color: #666; font-size: 12px;">Sent from FitGlue Server</p>
  `;
}

/**
 * Main handler - triggered by Pub/Sub message from Cloud Scheduler.
 */
// eslint-disable-next-line @typescript-eslint/no-unused-vars
export async function registrationSummaryHandler(
    _event: CloudEvent<{ message: { data: string } }>
): Promise<void> {
    // eslint-disable-next-line no-console
    console.log('Registration summary handler triggered');

    try {
        // Calculate yesterday's date range (UTC midnight to midnight)
        const now = new Date();
        const endDate = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
        const startDate = new Date(endDate.getTime() - 24 * 60 * 60 * 1000);

        const dateStr = startDate.toISOString().split('T')[0];
        // eslint-disable-next-line no-console
        console.log(`Fetching registrations from ${startDate.toISOString()} to ${endDate.toISOString()}`);

        // Query users created in the date range
        const users = await userStore.findByCreationDateRange(startDate, endDate);
        // eslint-disable-next-line no-console
        console.log(`Found ${users.length} users created in range`);

        // Fetch email addresses from Firebase Auth
        const userSummaries: UserSummary[] = [];
        for (const user of users) {
            try {
                const authUser = await auth.getUser(user.userId);
                userSummaries.push({
                    email: authUser.email || 'No email',
                    userId: user.userId,
                    createdAt: user.createdAt || new Date(),
                    accessEnabled: user.accessEnabled || false,
                });
            } catch (err) {
                // eslint-disable-next-line no-console
                console.warn(`Could not fetch auth user for ${user.userId}:`, err);
                userSummaries.push({
                    email: 'Unknown',
                    userId: user.userId,
                    createdAt: user.createdAt || new Date(),
                    accessEnabled: user.accessEnabled || false,
                });
            }
        }

        // Sort by creation date (newest first)
        userSummaries.sort((a, b) => b.createdAt.getTime() - a.createdAt.getTime());

        // Build email content
        const htmlContent = buildEmailContent(userSummaries, dateStr);

        // Get email credentials and send
        const emailPassword = await getEmailPassword();

        const transporter = nodemailer.createTransport({
            host: 'smtp.gmail.com',
            port: 587,
            secure: false, // Use STARTTLS
            auth: {
                user: SENDER_EMAIL,
                pass: emailPassword,
            },
        });

        const subject = userSummaries.length === 0
            ? `[FitGlue] No new registrations - ${dateStr}`
            : `[FitGlue] ${userSummaries.length} new registration${userSummaries.length === 1 ? '' : 's'} - ${dateStr}`;

        await transporter.sendMail({
            from: `"FitGlue System" <${SENDER_EMAIL}>`,
            to: RECIPIENT_EMAIL,
            subject,
            html: htmlContent,
        });

        // eslint-disable-next-line no-console
        console.log(`Email sent successfully to ${RECIPIENT_EMAIL}`);

    } catch (error) {
        // eslint-disable-next-line no-console
        console.error('Failed to send registration summary email:', error);
        throw error; // Rethrow to trigger retry
    }
}
