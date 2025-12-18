export enum ActivitySource {
  HEVY = 'hevy',
  KEISER = 'keiser',
  TEST = 'test'
}

export interface ActivityPayload {
  source: ActivitySource;
  userId: string;
  timestamp: string; // ISO 8601
  originalPayload: any; // The raw webhook/SDK data
  metadata?: Record<string, string>; // Optional traces
}
