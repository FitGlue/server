import { CloudEventSource } from '@fitglue/shared/dist/types/pb/events';
import { ActivitySource } from '@fitglue/shared/dist/types/pb/activity';

describe('File Upload Handler', () => {
  describe('Source Constants', () => {
    it('should have FILE_UPLOAD source enum defined', () => {
      expect(ActivitySource.SOURCE_FILE_UPLOAD).toBe(5);
      expect(CloudEventSource.CLOUD_EVENT_SOURCE_FILE_UPLOAD).toBe(8);
    });
  });
});
