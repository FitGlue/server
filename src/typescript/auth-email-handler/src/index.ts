/**
 * Auth Email Handler
 *
 * Sends branded authentication emails (verification, password reset,
 * email change) using the FitGlue shared email template library
 * and Firebase Admin SDK for generating action links.
 *
 * Routes:
 *   POST /send-verification     — Send email verification (authenticated)
 *   POST /send-password-reset   — Send password reset (unauthenticated, by email)
 *   POST /send-email-change     — Send email change verification (authenticated)
 *   POST /send-welcome          — Send welcome email (authenticated)
 */

import { createCloudFunction, FrameworkHandler } from '@fitglue/shared/framework';
import { HttpError } from '@fitglue/shared/errors';
import {
    verifyEmailTemplate,
    passwordResetTemplate,
    welcomeTemplate,
    changeEmailTemplate,
} from '@fitglue/shared/email';
import * as admin from 'firebase-admin';
import * as nodemailer from 'nodemailer';

// ─── Config ─────────────────────────────────────────────────────────

const SENDER_EMAIL = 'system@fitglue.tech';
const BASE_URL = 'https://fitglue.tech';

// Action code settings tell Firebase where to redirect after email actions
const actionCodeSettings = {
    url: `${BASE_URL}/auth/verify-email`,
    handleCodeInApp: false,
};

// ─── Email Transport ────────────────────────────────────────────────

function getEmailPassword(): string {
    const password = process.env.EMAIL_APP_PASSWORD;
    if (!password) {
        throw new Error('EMAIL_APP_PASSWORD environment variable is not set');
    }
    return password;
}

function createTransport() {
    return nodemailer.createTransport({
        host: 'smtp.gmail.com',
        port: 587,
        secure: false,
        auth: {
            user: SENDER_EMAIL,
            pass: getEmailPassword(),
        },
    });
}

async function sendEmail(to: string, subject: string, html: string): Promise<void> {
    const transporter = createTransport();
    await transporter.sendMail({
        from: `"FitGlue" <${SENDER_EMAIL}>`,
        to,
        subject,
        html,
    });
}

// ─── Route Handlers ─────────────────────────────────────────────────

/**
 * POST /send-verification
 * Generates a verification link and sends branded email.
 * Requires authentication (user must be logged in).
 */
async function handleSendVerification(
    userId: string,
    logger: { info: (msg: string, meta?: Record<string, unknown>) => void }
): Promise<Record<string, unknown>> {
    const auth = admin.auth();
    const user = await auth.getUser(userId);

    if (!user.email) {
        throw new HttpError(400, 'User has no email address');
    }

    if (user.emailVerified) {
        throw new HttpError(400, 'Email is already verified');
    }

    const verificationLink = await auth.generateEmailVerificationLink(
        user.email,
        { ...actionCodeSettings, url: `${BASE_URL}/auth/verify-email` }
    );

    await sendEmail(
        user.email,
        'Verify your FitGlue email',
        verifyEmailTemplate(verificationLink)
    );

    logger.info('Verification email sent', { userId, email: user.email });
    return { success: true, message: 'Verification email sent' };
}

/**
 * POST /send-password-reset
 * Generates a password reset link and sends branded email.
 * Unauthenticated — accepts email in request body.
 */
async function handleSendPasswordReset(
    body: { email?: string },
    logger: { info: (msg: string, meta?: Record<string, unknown>) => void }
): Promise<Record<string, unknown>> {
    const email = body?.email?.trim()?.toLowerCase();
    if (!email) {
        throw new HttpError(400, 'Email address is required');
    }

    // Always return success to prevent email enumeration attacks
    try {
        const auth = admin.auth();
        const resetLink = await auth.generatePasswordResetLink(
            email,
            { ...actionCodeSettings, url: `${BASE_URL}/auth/reset-password` }
        );

        await sendEmail(
            email,
            'Reset your FitGlue password',
            passwordResetTemplate(resetLink)
        );

        logger.info('Password reset email sent', { email });
    } catch (_error) {
        // Silently handle user-not-found to prevent enumeration
        logger.info('Password reset requested for unknown email', { email });
    }

    return { success: true, message: 'If an account exists with this email, a reset link has been sent' };
}

/**
 * POST /send-email-change
 * Generates an email change verification link and sends branded email.
 * Requires authentication (user must be logged in).
 */
async function handleSendEmailChange(
    userId: string,
    body: { newEmail?: string },
    logger: { info: (msg: string, meta?: Record<string, unknown>) => void }
): Promise<Record<string, unknown>> {
    const newEmail = body?.newEmail?.trim()?.toLowerCase();
    if (!newEmail) {
        throw new HttpError(400, 'New email address is required');
    }

    const auth = admin.auth();
    const verifyLink = await auth.generateVerifyAndChangeEmailLink(
        newEmail,
        newEmail,
        { ...actionCodeSettings, url: `${BASE_URL}/auth/verify-email-change` }
    );

    await sendEmail(
        newEmail,
        'Confirm your new FitGlue email',
        changeEmailTemplate(verifyLink, newEmail)
    );

    logger.info('Email change verification sent', { userId, newEmail });
    return { success: true, message: 'Verification email sent to new address' };
}

/**
 * POST /send-welcome
 * Sends a welcome email after email verification.
 * Requires authentication.
 */
async function handleSendWelcome(
    userId: string,
    logger: { info: (msg: string, meta?: Record<string, unknown>) => void }
): Promise<Record<string, unknown>> {
    const auth = admin.auth();
    const user = await auth.getUser(userId);

    if (!user.email) {
        throw new HttpError(400, 'User has no email address');
    }

    await sendEmail(
        user.email,
        'Welcome to FitGlue! 🎉',
        welcomeTemplate()
    );

    logger.info('Welcome email sent', { userId, email: user.email });
    return { success: true, message: 'Welcome email sent' };
}

// ─── Main Handler ───────────────────────────────────────────────────

const handler: FrameworkHandler = async (req, ctx) => {
    const path = req.path;
    const method = req.method;

    if (method !== 'POST') {
        throw new HttpError(405, 'Method not allowed');
    }

    // Password reset is the only unauthenticated route
    if (path.endsWith('/send-password-reset')) {
        return handleSendPasswordReset(
            req.body as { email?: string },
            ctx.logger
        );
    }

    // All other routes require authentication
    if (!ctx.userId) {
        throw new HttpError(401, 'Unauthorized');
    }

    if (path.endsWith('/send-verification')) {
        return handleSendVerification(ctx.userId, ctx.logger);
    }

    if (path.endsWith('/send-email-change')) {
        return handleSendEmailChange(
            ctx.userId,
            req.body as { newEmail?: string },
            ctx.logger
        );
    }

    if (path.endsWith('/send-welcome')) {
        return handleSendWelcome(ctx.userId, ctx.logger);
    }

    throw new HttpError(404, 'Not found');
};

// ─── Export ─────────────────────────────────────────────────────────

export const authEmailHandler = createCloudFunction(handler, {
    allowUnauthenticated: true, // Password reset needs unauthenticated access; handler checks ctx.userId for protected routes
});
