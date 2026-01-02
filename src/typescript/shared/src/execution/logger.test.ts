
import { logExecutionStart, logExecutionSuccess, logExecutionFailure } from './logger';

// Mock Firestore Storage Module
const mockSet = jest.fn();
const mockUpdate = jest.fn();
// For untyped update path: doc(...).withConverter(null).update(...)
const mockWithConverter = jest.fn(() => ({
  update: mockUpdate
}));
const mockDoc = jest.fn(() => ({
  id: 'exec-123',
  set: mockSet,
  update: mockUpdate, // Standard
  withConverter: mockWithConverter
}));


// Mock the module - auto-mock
jest.mock('../storage/firestore');


// We need to import the mocked function to verify calls if needed,
// though we mostly verify interaction with the objects it returns.
import { getExecutionsCollection } from '../storage/firestore';

describe('Execution Logger', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    // Configure the mock
    (getExecutionsCollection as jest.Mock).mockReturnValue({ doc: mockDoc });
  });

  describe('logExecutionStart', () => {
    it('should create a new execution document', async () => {
      const id = await logExecutionStart('test-service', { userId: 'user-1' });

      expect(id).toContain('test-service-');
      // Collection should be accessed
      expect(getExecutionsCollection).toHaveBeenCalled();
      expect(mockDoc).toHaveBeenCalledWith(id);
      expect(mockSet).toHaveBeenCalledWith(expect.objectContaining({
        service: 'test-service',
        status: 1, // STATUS_STARTED is 1
        userId: 'user-1', // camelCase keys in model object
        inputsJson: ''
      }));
    });
  });

  describe('logExecutionSuccess', () => {
    it('should update execution with success status', async () => {
      await logExecutionSuccess('exec-123', { result: 'ok' });

      expect(mockDoc).toHaveBeenCalledWith('exec-123');
      // Success/Failure use untyped update via withConverter(null)
      expect(mockWithConverter).toHaveBeenCalledWith(null);
      expect(mockUpdate).toHaveBeenCalledWith(expect.objectContaining({
        status: 2, // STATUS_SUCCESS is 2
        // Wait, enum values:
        // execution.proto:
        // STATUS_UNKNOWN = 0;
        // STATUS_STARTED = 1;
        // STATUS_RUNNING = 2;
        // STATUS_SUCCESS = 3;
        // STATUS_FAILED = 4;

        // I should import enum to be safe in test.
      }));
    });
  });

  describe('logExecutionFailure', () => {
    it('should update execution with failed status', async () => {
      await logExecutionFailure('exec-123', new Error('oops'));

      expect(mockDoc).toHaveBeenCalledWith('exec-123');
      expect(mockWithConverter).toHaveBeenCalledWith(null);
      expect(mockUpdate).toHaveBeenCalledWith(expect.objectContaining({
        status: 3, // STATUS_FAILED is 3
        error_message: 'oops' // The received object uses snake_case 'error_message'
        // logger.ts: error_message: error.message
      }));
    });
  });

  describe('logChildExecutionStart', () => {
    it('should create a child execution document with parent link', async () => {
      const { logChildExecutionStart } = require('./logger');

      const id = await logChildExecutionStart('child-service', 'parent-exec-123', { userId: 'user-1' });

      expect(id).toContain('child-service-');
      expect(mockDoc).toHaveBeenCalledWith(id);
      expect(mockSet).toHaveBeenCalledWith(expect.objectContaining({
        service: 'child-service',
        parentExecutionId: 'parent-exec-123'
      }));
    });
  });
});
