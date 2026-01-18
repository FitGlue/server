
import * as admin from 'firebase-admin';
import { adminDb as db } from "../firebase";
import { UserService } from '@fitglue/shared/dist/domain/services/user';
import { ApiKeyService } from '@fitglue/shared/dist/domain/services/apikey';
import { UserStore, ActivityStore, ApiKeyStore } from '@fitglue/shared/dist/storage/firestore';
import { generateOAuthState } from '@fitglue/shared';
import * as crypto from 'crypto';
import { randomUUID } from 'crypto';

const userStore = new UserStore(db);
const activityStore = new ActivityStore(db);
const apiKeyStore = new ApiKeyStore(db);

const userService = new UserService(userStore, activityStore);
const apiKeyService = new ApiKeyService(apiKeyStore);

export function registerUserTools(registerTool: (tool: any, handler: (args: any) => Promise<any>) => void) {

  // --- user_list ---
  registerTool(
    {
      name: "user_list",
      description: "List all users in the system with key details.",
      inputSchema: {
        type: "object",
        properties: {},
      },
    },
    async () => {
      const snapshot = await db.collection('users').get();
      if (snapshot.empty) return { message: "No users found" };

      const users = snapshot.docs.map(doc => {
        const data = doc.data();
        return {
          id: doc.id,
          created_at: data.created_at || data.createdAt, // keep raw timestamp object for now
          integrations: {
            hevy: !!(data.integrations?.hevy?.api_key || data.integrations?.hevy?.apiKey),
            strava: !!data.integrations?.strava?.enabled,
            fitbit: !!data.integrations?.fitbit?.enabled
          },
          pipeline_count: data.pipelines?.length || 0,
          // Tier info
          tier: data.tier || 'free',
          trial_ends_at: data.trial_ends_at,
          is_admin: data.is_admin || false,
          sync_count_this_month: data.sync_count_this_month || 0,
          stripe_customer_id: data.stripe_customer_id || null,
        };
      });
      return users;
    }
  );

  // --- user_get ---
  registerTool(
    {
      name: "user_get",
      description: "Get full details of a specific user.",
      inputSchema: {
        type: "object",
        properties: {
          userId: { type: "string" },
        },
        required: ["userId"],
      },
    },
    async ({ userId }: { userId: string }) => {
      const doc = await db.collection('users').doc(userId).get();
      if (!doc.exists) throw new Error(`User ${userId} not found`);
      return { id: doc.id, ...doc.data() };
    }
  );

  // --- user_create ---
  registerTool(
    {
      name: "user_create",
      description: "Create a new user and generate an Ingress API Key.",
      inputSchema: {
        type: "object",
        properties: {
          userId: { type: "string", description: "Optional custom UUID" },
          createIngressKey: { type: "boolean", default: true },
          keyLabel: { type: "string", default: "Default Key" },
        },
      },
    },
    async ({ userId, createIngressKey = true, keyLabel = "Default Key" }: { userId?: string, createIngressKey?: boolean, keyLabel?: string }) => {
      const finalUserId = userId || randomUUID();
      await userService.createUser(finalUserId);

      let ingressKey = null;
      if (createIngressKey) {
        const token = `fg_sk_${crypto.randomBytes(32).toString('hex')}`;
        const hash = crypto.createHash('sha256').update(token).digest('hex');

        await apiKeyService.create(hash, {
          label: keyLabel,
          scopes: ['read:activity'],
          userId: finalUserId,
          createdAt: new Date()
        });
        ingressKey = token;
      }

      return {
        userId: finalUserId,
        message: "User created successfully",
        ingressKey: ingressKey
      };
    }
  );

  // --- user_create_auth ---
  registerTool(
    {
      name: "user_create_auth",
      description: "Create a Firebase Auth user for an existing Firestore User ID.",
      inputSchema: {
        type: "object",
        properties: {
          userId: { type: "string" },
          email: { type: "string" },
          password: { type: "string" },
        },
        required: ["userId", "email", "password"],
      },
    },
    async ({ userId, email, password }: { userId: string, email: string, password: string }) => {
      // Verify user exists first
      const doc = await db.collection('users').doc(userId).get();
      if (!doc.exists) throw new Error(`Firestore User ${userId} not found`);

      const userRecord = await admin.auth().createUser({
        uid: userId,
        email,
        password,
      });
      return { uid: userRecord.uid, email: userRecord.email, message: "Auth user created" };
    }
  );

  // --- user_delete ---
  registerTool(
    {
      name: "user_delete",
      description: "Delete a user permanently.",
      inputSchema: {
        type: "object",
        properties: {
          userId: { type: "string" },
          confirm: { type: "boolean", description: "Must be true to proceed" }
        },
        required: ["userId", "confirm"],
      },
    },
    async ({ userId, confirm }: { userId: string, confirm: boolean }) => {
      if (!confirm) throw new Error("Confirmation required");
      await db.collection('users').doc(userId).delete();
      return { message: `User ${userId} deleted` };
    }
  );

  // --- user_clean ---
  registerTool(
    {
      name: "user_clean",
      description: "DELETE ALL USERS. EXTREME CAUTION.",
      inputSchema: {
        type: "object",
        properties: {
          confirm: { type: "string", description: "Must be 'DELETE ALL'" }
        },
        required: ["confirm"],
      },
    },
    async ({ confirm }: { confirm: string }) => {
      if (confirm !== "DELETE ALL") throw new Error("Invalid confirmation string");

      const snapshot = await db.collection('users').get();
      if (snapshot.empty) return { message: "No users to delete" };

      const batch = db.batch();
      snapshot.docs.forEach(doc => batch.delete(doc.ref));
      await batch.commit();

      return { message: `Deleted ${snapshot.size} users` };
    }
  );

  // --- user_connect ---
  registerTool(
    {
      name: "user_connect",
      description: "Generate OAuth authorization URL for a user (Strava/Fitbit).",
      inputSchema: {
        type: "object",
        properties: {
          userId: { type: "string" },
          provider: { type: "string", enum: ["strava", "fitbit"] },
          clientId: { type: "string" },
          env: { type: "string", enum: ["dev", "test", "prod"], default: "dev" }
        },
        required: ["userId", "provider", "clientId"],
      },
    },
    async ({ userId, provider, clientId, env = "dev" }: { userId: string, provider: string, clientId: string, env: string }) => {
      const userDoc = await db.collection('users').doc(userId).get();
      if (!userDoc.exists) throw new Error(`User ${userId} not found`);

      const baseUrl = env === 'prod' ? 'https://fitglue.tech' : `https://${env}.fitglue.tech`;
      const state = await generateOAuthState(userId);

      let authUrl: string;
      if (provider === 'strava') {
        authUrl = `https://www.strava.com/oauth/authorize?` +
          `client_id=${clientId}&` +
          `redirect_uri=${encodeURIComponent(`${baseUrl}/auth/strava/callback`)}&` +
          `response_type=code&` +
          `scope=read,activity:read_all,activity:write&` +
          `state=${state}`;
      } else {
        authUrl = `https://www.fitbit.com/oauth2/authorize?` +
          `client_id=${clientId}&` +
          `redirect_uri=${encodeURIComponent(`${baseUrl}/auth/fitbit/callback`)}&` +
          `response_type=code&` +
          `scope=${encodeURIComponent('activity heartrate profile location')}&` +
          `state=${state}`;
      }
      return { authUrl, state };
    }
  );

  // --- user_configure_hevy ---
  registerTool(
    {
      name: "user_configure_hevy",
      description: "Configure Hevy integration for a user.",
      inputSchema: {
        type: "object",
        properties: {
          userId: { type: "string" },
          apiKey: { type: "string" }
        },
        required: ["userId", "apiKey"],
      },
    },
    async ({ userId, apiKey }: { userId: string, apiKey: string }) => {
      const userDoc = await db.collection('users').doc(userId).get();
      if (!userDoc.exists) throw new Error(`User ${userId} not found`);

      await userService.setHevyIntegration(userId, apiKey);
      return { message: "Hevy integration configured" };
    }
  );

  // --- user_update ---
  registerTool(
    {
      name: "user_update",
      description: "Update user tier, admin status, or integration credentials.",
      inputSchema: {
        type: "object",
        properties: {
          userId: { type: "string" },
          // Tier management fields
          tier: { type: "string", enum: ["free", "pro"], description: "Set user tier directly" },
          isAdmin: { type: "boolean", description: "Grant/revoke admin (Pro) access" },
          trialEndsAt: { type: ["string", "null"], description: "ISO date or null to clear trial" },
          // Integration fields
          strava: {
            type: "object",
            properties: {
              accessToken: { type: "string" },
              refreshToken: { type: "string" },
              expiresAt: { type: "number" },
              athleteId: { type: "number" }
            }
          },
          fitbit: {
            type: "object",
            properties: {
              accessToken: { type: "string" },
              refreshToken: { type: "string" },
              expiresAt: { type: "number" },
              fitbitUserId: { type: "string" }
            }
          }
        },
        required: ["userId"]
      }
    },
    async ({ userId, tier, isAdmin, trialEndsAt, strava, fitbit }: any) => {
      // Handle tier management updates
      const tierUpdates: Record<string, any> = {};
      if (tier !== undefined) tierUpdates.tier = tier;
      if (isAdmin !== undefined) tierUpdates.is_admin = isAdmin;
      if (trialEndsAt !== undefined) {
        tierUpdates.trial_ends_at = trialEndsAt ? new Date(trialEndsAt) : null;
      }

      if (Object.keys(tierUpdates).length > 0) {
        await db.collection('users').doc(userId).update(tierUpdates);
      }

      // Handle integration updates
      if (strava) {
        await userService.setStravaIntegration(
          userId,
          strava.accessToken,
          strava.refreshToken,
          strava.expiresAt,
          strava.athleteId
        );
      }
      if (fitbit) {
        await userService.setFitbitIntegration(
          userId,
          fitbit.accessToken,
          fitbit.refreshToken,
          fitbit.expiresAt,
          fitbit.fitbitUserId
        );
      }
      return { message: "User updated", tierUpdates };
    }
  );

  // --- fitbit_subscribe ---
  // Requires 'createFitbitClient' logic.
  // Note: createFitbitClient needs secrets.
  // We'll skip deep implementation of fitbit_subscribe for now as it's complex with GSM.
  // Or... can allow it if secrets are available in env.
  // Let's implement stub or basic version.

  // --- pipeline_add ---
  registerTool(
    {
      name: "pipeline_add",
      description: "Add a processing pipeline to a user.",
      inputSchema: {
        type: "object",
        properties: {
          userId: { type: "string" },
          source: { type: "string" },
          enrichers: {
            type: "array",
            items: {
              type: "object",
              properties: {
                providerType: { type: "number" },
                inputs: { type: "object" } // Generic map for inputs
              },
              required: ["providerType"]
            }
          },
          destinations: { type: "array", items: { type: "string" } }
        },
        required: ["userId", "source", "enrichers", "destinations"]
      }
    },
    async ({ userId, source, enrichers, destinations }: any) => {
      const id = await userService.addPipeline(userId, '', source, enrichers, destinations);
      return { pipelineId: id, message: "Pipeline added" };
    }
  );

  // --- pipeline_remove ---
  registerTool(
    {
      name: "pipeline_remove",
      description: "Remove a pipeline.",
      inputSchema: {
        type: "object",
        properties: {
          userId: { type: "string" },
          pipelineId: { type: "string" }
        },
        required: ["userId", "pipelineId"]
      }
    },
    async ({ userId, pipelineId }: { userId: string, pipelineId: string }) => {
      await userService.removePipeline(userId, pipelineId);
      return { message: "Pipeline removed" };
    }
  );
}
