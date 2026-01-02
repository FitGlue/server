import { ExecutionRecord, ExecutionStatus } from '../types/pb/execution';
import { getExecutionsCollection } from '../storage/firestore';

export interface ExecutionOptions {
  userId?: string;
  testRunId?: string;
  triggerType?: string;
  inputs?: any;
}

/**
 * Log the start of a function execution
 * @returns execution ID
 */
export async function logExecutionStart(
  service: string,
  opts: ExecutionOptions = {}
): Promise<string> {
  const execId = `${service}-${Date.now()}`;
  const now = new Date();

  const record: ExecutionRecord = {
    executionId: execId,
    service,
    status: ExecutionStatus.STATUS_STARTED,
    timestamp: now,
    startTime: now,
    endTime: undefined,
    userId: opts.userId || '',
    testRunId: opts.testRunId || '',
    triggerType: opts.triggerType || '',
    errorMessage: '',
    inputsJson: opts.inputs ? JSON.stringify(opts.inputs) : '',
    outputsJson: '',
    parentExecutionId: ''
  };

  await getExecutionsCollection().doc(execId).set(record);
  return execId;
}

/**
 * Log the start of a child function execution linked to a parent
 * @returns execution ID
 */
export async function logChildExecutionStart(
  service: string,
  parentExecutionID: string,
  opts: ExecutionOptions = {}
): Promise<string> {
  const execId = `${service}-${Date.now()}`;
  const now = new Date();

  // Fully typed record
  const record: ExecutionRecord = {
    executionId: execId,
    service,
    status: ExecutionStatus.STATUS_STARTED,
    timestamp: now,
    startTime: now,
    endTime: undefined,
    userId: opts.userId || '',
    testRunId: opts.testRunId || '',
    triggerType: opts.triggerType || '',
    parentExecutionId: parentExecutionID,
    errorMessage: '',
    inputsJson: opts.inputs ? JSON.stringify(opts.inputs) : '',
    outputsJson: ''
  };

  await getExecutionsCollection().doc(execId).set(record);
  return execId;
}

/**
 * Log successful completion of a function execution
 */
export async function logExecutionSuccess(
  execId: string,
  outputs?: any
): Promise<void> {
  const now = new Date();

  // Update with partial data
  // Using set with merge or update with typed data?
  // getExecutionsCollection is Typed. update() expects dictionary of keys.
  // We can use update since our converter handles full objects in set but update bypasses specialized transformation for specific fields?
  // Our converter `toFirestore` handles FULL record.
  // For `update`, SDK implementation of `.withConverter` allows passing partial data but validates keys against T? NO/YES?
  // Actually, standard `DocumentReference.update(data: UpdateData<T>)` works if T is defined.
  // BUT the Keys of UpdateData<T> match T's keys (camelCase).
  // AND `withConverter` implies the converter handles the transformation.
  // Does `firebase-admin` call `toFirestore` for `update`?
  // YES, if `toFirestore(model, options)` signature is present? Or `toFirestore(model)`.
  // It merges the result.
  // So if we pass `{ status: ... }`, converter receives `{ status: ... }`.
  // Our converter implementation: `toFirestore(model: ExecutionRecord)`.
  // It accesses properties: `model.executionId`, `model.service`.
  // If we pass a PARTIAL object, those properties are UNDEFINED.
  // `toFirestore` returns `{ execution_id: undefined, service: undefined, status: ... }`.
  // Firestore treats `undefined` as "Ignore/Don't Write" usually?
  // NO. `IgnoreUndefinedProperties` setting on Firestore instance. Default is ERROR?
  // Default in admin SDK is usually to ignore or error.
  // IF it errors or writes null/undefined, we have a problem.
  // Our converter implementation does NOT handle partials gracefully if it blindly accesses props.

  // FIX: Access `getExecutionsCollection()` with explicit `withConverter(null)` for updates, OR update Converter to handle partials.
  // Updating converter to handle partials is best but requires changing the signature in `converters.ts`.
  // `FirestoreDataConverter` types enforce `toFirestore(model: T): DocumentData`. T is the full type.
  // So official type does NOT support partials easily.
  // Standard pattern: Use `set(..., {merge: true})`.
  // `set` calls `toFirestore(model, {merge:true})`.
  // If `merge: true`, `toFirestore` still receives the input object.
  // If input is partial, we must implement `toFirestore` to handle it.

  // SIMPLER FIX: Use untyped update (with `withConverter(null)`) and manual snake_case mapping for these update operations.
  // This is pragmatic for "Success/Failure" updates which are specific.

  const updates: any = {
    status: ExecutionStatus.STATUS_SUCCESS, // Will be int?
    // Wait, if we use untyped update, we must match what we want in DB.
    // If we want Int in DB, pass Int.
    // ExecutionStatus.STATUS_SUCCESS is Int.
    timestamp: now,
    end_time: now
  };

  if (outputs) {
    updates.outputs_json = JSON.stringify(outputs);
  }

  // Untyped update
  await getExecutionsCollection().doc(execId).withConverter(null).update(updates);
}

/**
 * Log failed execution of a function
 */
export async function logExecutionFailure(
  execId: string,
  error: Error
): Promise<void> {
  const now = new Date();

  const updates = {
    status: ExecutionStatus.STATUS_FAILED,
    timestamp: now,
    end_time: now, // snake_case
    error_message: error.message // snake_case
  };

  await getExecutionsCollection().doc(execId).withConverter(null).update(updates);
}

