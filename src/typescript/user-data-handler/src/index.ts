// Module-level imports for smart pruning
import { createCloudFunction, FirebaseAuthStrategy, FrameworkHandler, db } from '@fitglue/shared/framework';
import { HttpError } from '@fitglue/shared/errors';
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

// eslint-disable-next-line complexity
export const handler: FrameworkHandler = async (req, ctx) => {
    if (!ctx.userId) {
        throw new HttpError(401, 'Unauthorized');
    }

    const path = req.path;
    const method = req.method;
    const userId = ctx.userId;

    // --- Counter Routes ---
    if (path.includes('/counters')) {
        const countersCollection = db.collection('users').doc(userId).collection('counters');

        if (method === 'GET') {
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
            return { counters };
        }

        if (method === 'POST') {
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
        }

        if (method === 'DELETE') {
            // Extract counter ID from path: /counters/:id or /api/user-data/counters/:id
            const segments = path.split('/').filter(s => s.length > 0);
            const countersIndex = segments.findIndex(s => s === 'counters');
            const counterId = segments[countersIndex + 1];

            if (!counterId) {
                throw new HttpError(400, 'Missing counter id');
            }

            await countersCollection.doc(decodeURIComponent(counterId)).delete();
            ctx.logger.info('Deleted counter', { userId, counterId });
            return { success: true };
        }
    }

    // --- Personal Records Routes ---
    if (path.includes('/personal-records')) {
        const recordsCollection = db.collection('users').doc(userId).collection('personal_records');

        if (method === 'GET') {
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
        }

        if (method === 'POST') {
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
        }

        if (method === 'DELETE') {
            // Extract record type from path: /personal-records/:type
            const segments = path.split('/').filter(s => s.length > 0);
            const recordsIndex = segments.findIndex(s => s === 'personal-records');
            const recordType = segments[recordsIndex + 1];

            if (!recordType) {
                throw new HttpError(400, 'Missing record type');
            }

            await recordsCollection.doc(decodeURIComponent(recordType)).delete();
            ctx.logger.info('Deleted personal record', { userId, recordType });
            return { success: true };
        }
    }

    throw new HttpError(404, 'Not found');
};

// Export the wrapped function
export const userDataHandler = createCloudFunction(handler, {
    auth: {
        strategies: [new FirebaseAuthStrategy()]
    },
    skipExecutionLogging: true
});
