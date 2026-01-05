import { createCloudFunction, FrameworkContext, FirebaseAuthStrategy } from '@fitglue/shared';
import { Request, Response } from 'express';

export const handler = async (req: Request, res: Response, ctx: FrameworkContext) => {
  const activityStore = ctx.stores.activities;

  // Auth check
  if (!ctx.userId) {
    res.status(401).json({ error: 'Unauthorized' });
    return;
  }

  try {
    // GET /stats -> { synchronized_count: N }
    if (req.path === '/stats' || req.query.mode === 'stats') {
      // Logic: Start of current week (Monday)
      const now = new Date();
      const day = now.getDay(); // 0 is Sunday
      const diff = now.getDate() - day + (day === 0 ? -6 : 1); // adjust when day is sunday
      const monday = new Date(now.setDate(diff));
      monday.setHours(0, 0, 0, 0);

      const count = await activityStore.countSynchronized(ctx.userId, monday);
      res.status(200).json({ synchronized_count: count });
      return;
    }

    // GET /:id -> Single activity
    // Note: Cloud Functions HTTP often passes path parts differently depending on deployment
    // We'll check rudimentary path parsing or query param 'id'
    const id = req.query.id as string;
    if (id) {
      const activity = await activityStore.getSynchronized(ctx.userId, id);
      if (!activity) {
        res.status(404).json({ error: 'Not found' });
        return;
      }
      res.status(200).json({ activity });
      return;
    }

    // GET / -> List
    const limit = parseInt(req.query.limit as string || '20', 10);
    const activities = await activityStore.listSynchronized(ctx.userId, limit);
    res.status(200).json({ activities });

  } catch (e) {
    ctx.logger.error('Failed to handle activities request', { error: e });
    res.status(500).json({ error: 'Internal Server Error' });
  }
};

export const activitiesHandler = createCloudFunction(handler, {
  auth: {
    strategies: [
      new FirebaseAuthStrategy()
    ]
  }
});
