// Module-level imports for smart pruning
import { createCloudFunction, FirebaseAuthStrategy, FrameworkHandler, db } from '@fitglue/shared/framework';
import { HttpError } from '@fitglue/shared/errors';
import { routeRequest, RouteMatch, RoutableRequest } from '@fitglue/shared/routing';
import { Timestamp } from 'firebase-admin/firestore';

/**
 * User Data Handler - CRUD operations for enricher-stored data
 *
 * Endpoints:
 * - GET  /counters              - List all counters
 * - POST /counters              - Create or update a counter
 * - DELETE /counters/:id        - Delete a counter
 * - GET  /personal-records      - List all personal records
 * - POST /personal-records      - Create or update a personal record
 * - DELETE /personal-records/:type - Delete a personal record
 * - GET  /booster-data          - List all booster data
 * - GET  /booster-data/:id      - Get specific booster data
 * - POST /booster-data/:id      - Create or update booster data
 * - DELETE /booster-data/:id    - Delete booster data
 */

interface Counter {
    id: string;
    count: number;
    lastUpdated: string;
}

interface PersonalRecord {
    recordType: string;
    value: number;
    unit: string;
    activityId?: string;
    achievedAt?: string;
    activityType?: string;
    previousValue?: number;
    improvement?: number;
}

export const handler: FrameworkHandler = async (req, ctx) => {
    if (!ctx.userId) {
        throw new HttpError(401, 'Unauthorized');
    }

    const userId = ctx.userId;

    return await routeRequest(req as RoutableRequest, ctx, [
        // --- Counter Routes ---
        {
            method: 'GET',
            pattern: '*/counters',
            handler: async () => {
                const countersCollection = db.collection('users').doc(userId).collection('counters');
                const snapshot = await countersCollection.get();
                const counters: Counter[] = snapshot.docs.map(doc => {
                    const data = doc.data();
                    return {
                        id: doc.id,
                        count: data.count ?? 0,
                        lastUpdated: data.last_updated?.toDate?.()?.toISOString() ??
                            data.lastUpdated?.toDate?.()?.toISOString() ??
                            new Date().toISOString()
                    };
                });
                return counters;
            },
        },
        {
            method: 'POST',
            pattern: '*/counters',
            handler: async () => {
                const countersCollection = db.collection('users').doc(userId).collection('counters');
                const { id, count } = req.body as { id?: string; count?: number };
                if (!id) {
                    throw new HttpError(400, 'Missing counter id');
                }
                if (typeof count !== 'number') {
                    throw new HttpError(400, 'Missing or invalid count value');
                }

                await countersCollection.doc(id).set({
                    id,
                    count,
                    last_updated: Timestamp.now()
                });

                ctx.logger.info('Updated counter', { userId, counterId: id, count });
                return { success: true, counter: { id, count, lastUpdated: new Date().toISOString() } };
            },
        },
        {
            method: 'DELETE',
            pattern: '*/counters/:id',
            handler: async (match: RouteMatch) => {
                const countersCollection = db.collection('users').doc(userId).collection('counters');
                const counterId = match.params.id;

                await countersCollection.doc(decodeURIComponent(counterId)).delete();
                ctx.logger.info('Deleted counter', { userId, counterId });
                return { success: true };
            },
        },

        // --- Personal Records Routes ---
        {
            method: 'GET',
            pattern: '*/personal-records',
            handler: async () => {
                const recordsCollection = db.collection('users').doc(userId).collection('personal_records');
                const snapshot = await recordsCollection.get();
                const records: PersonalRecord[] = snapshot.docs.map(doc => {
                    const data = doc.data();
                    return {
                        recordType: doc.id,
                        value: data.value ?? 0,
                        unit: data.unit ?? '',
                        activityId: data.activity_id ?? data.activityId,
                        achievedAt: data.achieved_at?.toDate?.()?.toISOString() ??
                            data.achievedAt?.toDate?.()?.toISOString(),
                        activityType: data.activity_type ?? data.activityType,
                        previousValue: data.previous_value ?? data.previousValue,
                        improvement: data.improvement
                    };
                });
                return { records };
            },
        },
        {
            method: 'POST',
            pattern: '*/personal-records',
            handler: async () => {
                const recordsCollection = db.collection('users').doc(userId).collection('personal_records');
                const body = req.body as Partial<PersonalRecord>;
                const { recordType, value, unit } = body;

                if (!recordType) {
                    throw new HttpError(400, 'Missing record type');
                }
                if (typeof value !== 'number') {
                    throw new HttpError(400, 'Missing or invalid value');
                }
                if (!unit) {
                    throw new HttpError(400, 'Missing unit');
                }

                const recordData: Record<string, unknown> = {
                    record_type: recordType,
                    value,
                    unit,
                    achieved_at: Timestamp.now()
                };

                // Optional fields
                if (body.activityId) recordData.activity_id = body.activityId;
                if (body.activityType) recordData.activity_type = body.activityType;
                if (body.previousValue !== undefined) recordData.previous_value = body.previousValue;
                if (body.improvement !== undefined) recordData.improvement = body.improvement;

                await recordsCollection.doc(recordType).set(recordData);

                ctx.logger.info('Updated personal record', { userId, recordType, value });
                return {
                    success: true,
                    record: {
                        recordType,
                        value,
                        unit,
                        achievedAt: new Date().toISOString()
                    }
                };
            },
        },
        {
            method: 'DELETE',
            pattern: '*/personal-records/:type',
            handler: async (match: RouteMatch) => {
                const recordsCollection = db.collection('users').doc(userId).collection('personal_records');
                const recordType = match.params.type;

                await recordsCollection.doc(decodeURIComponent(recordType)).delete();
                ctx.logger.info('Deleted personal record', { userId, recordType });
                return { success: true };
            },
        },

        // --- Booster Data Routes ---
        {
            method: 'GET',
            pattern: '*/booster-data/:id',
            handler: async (match: RouteMatch) => {
                const boosterDataCollection = db.collection('users').doc(userId).collection('booster_data');
                const doc = await boosterDataCollection.doc(decodeURIComponent(match.params.id)).get();
                if (!doc.exists) {
                    return { data: {} };
                }
                return { data: doc.data() };
            },
        },
        {
            method: 'GET',
            pattern: '*/booster-data',
            handler: async () => {
                const boosterDataCollection = db.collection('users').doc(userId).collection('booster_data');
                const snapshot = await boosterDataCollection.get();
                const data: Record<string, unknown> = {};
                snapshot.docs.forEach(doc => {
                    data[doc.id] = doc.data();
                });
                return { data };
            },
        },
        {
            method: 'POST',
            pattern: '*/booster-data/:id',
            handler: async (match: RouteMatch) => {
                const boosterDataCollection = db.collection('users').doc(userId).collection('booster_data');
                const boosterId = match.params.id;

                const body = req.body as Record<string, unknown>;
                if (!body || typeof body !== 'object') {
                    throw new HttpError(400, 'Missing or invalid body');
                }

                // Merge update to support incremental updates
                await boosterDataCollection.doc(decodeURIComponent(boosterId)).set({
                    ...body,
                    last_updated: Timestamp.now()
                }, { merge: true });

                ctx.logger.info('Updated booster data', { userId, boosterId });
                return { success: true };
            },
        },
        {
            method: 'DELETE',
            pattern: '*/booster-data/:id',
            handler: async (match: RouteMatch) => {
                const boosterDataCollection = db.collection('users').doc(userId).collection('booster_data');
                const boosterId = match.params.id;

                await boosterDataCollection.doc(decodeURIComponent(boosterId)).delete();
                ctx.logger.info('Deleted booster data', { userId, boosterId });
                return { success: true };
            },
        },

        // --- Notification Preferences Routes ---
        {
            method: 'GET',
            pattern: '*/notification-preferences',
            handler: async () => {
                const userRef = db.collection('users').doc(userId);
                const doc = await userRef.get();
                if (!doc.exists) {
                    // Default preferences - all notifications enabled
                    return {
                        notifyPendingInput: true,
                        notifyPipelineSuccess: true,
                        notifyPipelineFailure: true
                    };
                }
                const data = doc.data();
                const prefs = data?.notification_preferences ?? {};
                return {
                    notifyPendingInput: prefs.notify_pending_input ?? true,
                    notifyPipelineSuccess: prefs.notify_pipeline_success ?? true,
                    notifyPipelineFailure: prefs.notify_pipeline_failure ?? true
                };
            },
        },
        {
            method: 'PATCH',
            pattern: '*/notification-preferences',
            handler: async () => {
                const userRef = db.collection('users').doc(userId);
                const body = req.body as {
                    notifyPendingInput?: boolean;
                    notifyPipelineSuccess?: boolean;
                    notifyPipelineFailure?: boolean;
                };

                const updates: Record<string, boolean> = {};
                if (typeof body.notifyPendingInput === 'boolean') {
                    updates['notification_preferences.notify_pending_input'] = body.notifyPendingInput;
                }
                if (typeof body.notifyPipelineSuccess === 'boolean') {
                    updates['notification_preferences.notify_pipeline_success'] = body.notifyPipelineSuccess;
                }
                if (typeof body.notifyPipelineFailure === 'boolean') {
                    updates['notification_preferences.notify_pipeline_failure'] = body.notifyPipelineFailure;
                }

                if (Object.keys(updates).length === 0) {
                    throw new HttpError(400, 'No valid preferences provided');
                }

                await userRef.update(updates);
                ctx.logger.info('Updated notification preferences', { userId, updates });
                return { success: true };
            },
        },
    ]);
};

// Export the wrapped function
export const userDataHandler = createCloudFunction(handler, {
    auth: {
        strategies: [new FirebaseAuthStrategy()]
    },
    skipExecutionLogging: true
});
