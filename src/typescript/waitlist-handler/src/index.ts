import * as functions from '@google-cloud/functions-framework';
import * as admin from 'firebase-admin';

// Initialize Firebase Admin (only once)
if (!admin.apps.length) {
  admin.initializeApp();
}

const db = admin.firestore();

export const waitlistHandler = async (req: functions.Request, res: functions.Response) => {
  // CORS Headers
  // Allow all origins for the waitlist (public)
  res.set('Access-Control-Allow-Origin', '*');
  res.set('Access-Control-Allow-Methods', 'POST, OPTIONS');
  res.set('Access-Control-Allow-Headers', 'Content-Type');
  // Cache preflight response for 3600s
  res.set('Access-Control-Max-Age', '3600');

  if (req.method === 'OPTIONS') {
    res.status(204).send('');
    return;
  }

  if (req.method !== 'POST') {
    res.status(405).send('Method Not Allowed');
    return;
  }

  const { email, website_url } = req.body;

  // --- Spam Protection (Honeypot) ---
  // If the hidden 'website_url' field is filled, it's likely a bot.
  if (website_url) {
    console.warn(`Spam detected: honeypot 'website_url' filled with "${website_url}". Email provided: "${email}"`);
    // Return success to fool the bot into thinking it succeeded vs trying again
    res.status(200).json({ success: true, message: 'Thanks for joining!' });
    return;
  }

  // --- Validation ---
  if (!email || typeof email !== 'string' || !isValidEmail(email)) {
    res.status(400).json({ error: 'Please provide a valid email address.' });
    return;
  }

  const normalizedEmail = email.toLowerCase();

  try {
    // --- Persistence ---
    // Use email as document ID to ensure uniqueness and simple lookups.
    // .create() will fail if the document already exists.
    await db.collection('waitlist').doc(normalizedEmail).create({
      email: normalizedEmail,
      source: 'web',
      createdAt: admin.firestore.FieldValue.serverTimestamp(),
      userAgent: req.get('User-Agent') || 'unknown',
      ip: req.ip || 'unknown'
    });

    console.log(`Waitlist entry added for: ${maskEmail(normalizedEmail)}`);
    res.status(200).json({ success: true, message: 'Thanks for joining!' });
  } catch (error: any) {
    if (error.code === 6 || error.message.includes('ALREADY_EXISTS')) {
      console.log(`Duplicate waitlist attempt for: ${maskEmail(normalizedEmail)}`);
      res.status(409).json({ success: false, error: "You're already on the waitlist" });
      return;
    }
    console.error('Error adding to waitlist:', error);
    res.status(500).json({ error: 'Internal Server Error' });
  }
};

function isValidEmail(email: string): boolean {
  // Basic email regex
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
}

function maskEmail(email: string): string {
  const parts = email.split('@');
  if (parts.length < 2) return '***';
  const name = parts[0];
  const domain = parts[1];
  return `${name.substring(0, 3)}***@${domain}`;
}
