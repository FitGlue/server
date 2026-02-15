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
import { registrationSummaryTemplate } from '@fitglue/shared/email';
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
 * Build the email HTML content using shared branded template.
 */
function buildEmailContent(users: UserSummary[], dateStr: string): string {
  return registrationSummaryTemplate(dateStr, users.map(u => ({
    email: u.email,
    accessEnabled: u.accessEnabled,
    createdAt: u.createdAt,
  })));
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
